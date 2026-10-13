package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"server-template/internal/delivery/http/response"
	"server-template/internal/pkg/apperr"
	"server-template/internal/pkg/observability"

	"github.com/labstack/echo/v5"
)

// RequestLoggerOptions configures RequestLogger.
type RequestLoggerOptions struct {
	// Debug also logs successful (status < 400) requests.
	Debug bool
}

// RequestLogger logs one entry per request after the response is final.
// Successful requests are logged only when opts.Debug is set; client errors
// log at Warn without a stack; server faults log at Error with the stack and
// the sanitized cause. Install it outside HandleErrors so the final error is
// already recorded in RequestInfo.
func RequestLogger(logger *slog.Logger, opts RequestLoggerOptions) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			err := next(c)
			logRequest(logger, opts, c, start, err)

			return err
		}
	}
}

func logRequest(logger *slog.Logger, opts RequestLoggerOptions, c *echo.Context, start time.Time, err error) {
	req := c.Request()
	ctx := req.Context()
	info := observability.RequestInfoFrom(ctx)

	var ae *apperr.Error
	if info != nil {
		ae = info.Err
	}
	if ae == nil && err != nil {
		ae = toAppError(err)
	}

	status := responseStatus(c, ae)
	if status < http.StatusBadRequest && ae == nil && !opts.Debug {
		return
	}

	// trace_id/span_id are added by the slog trace handler when OTel is on.
	attrs := []slog.Attr{
		slog.String("request_id", observability.CorrelationIDFromContext(ctx)),
		slog.String("method", req.Method),
		slog.String("path", req.URL.Path),
		slog.Int("status", status),
		slog.Duration("latency", time.Since(start)),
		slog.String("remote_ip", c.RealIP()),
		slog.String("user_agent", req.UserAgent()),
	}

	level := logLevel(ae, status)
	if level >= slog.LevelWarn {
		var body []byte
		if info != nil {
			body = info.Body
		}
		attrs = append(attrs, slog.Any("request", buildSanitizedRequestLog(req, body)))
		attrs = append(attrs, errorAttrs(ae, status, level == slog.LevelError)...)
	}

	logger.LogAttrs(ctx, level, "HTTP request", attrs...)
}

// responseStatus is the written status, or the one the error maps to when
// nothing has been written yet.
func responseStatus(c *echo.Context, ae *apperr.Error) int {
	if resp, _ := echo.UnwrapResponse(c.Response()); resp != nil && resp.Status != 0 {
		return resp.Status
	}
	if ae != nil {
		return response.StatusFor(ae.Kind)
	}

	return http.StatusOK
}

func logLevel(ae *apperr.Error, status int) slog.Level {
	switch {
	case ae != nil && ae.Kind.IsServerFault(), status >= http.StatusInternalServerError:
		return slog.LevelError
	case ae != nil, status >= http.StatusBadRequest:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// errorAttrs describes the error without leaking secrets: fields are shown
// only for client errors other than 401/403, the cause is free-text sanitized.
func errorAttrs(ae *apperr.Error, status int, withStack bool) []slog.Attr {
	if ae == nil {
		return []slog.Attr{slog.Any("response_error", buildResponseErrorLog(status, "HTTP_ERROR", http.StatusText(status), nil))}
	}

	var details any
	if len(ae.Fields) > 0 && status < http.StatusInternalServerError &&
		status != http.StatusUnauthorized && status != http.StatusForbidden {
		fields := make([]any, 0, len(ae.Fields))
		for _, f := range ae.Fields {
			fields = append(fields, map[string]any{"field": f.Field, "description": sanitizeString(f.Description)})
		}
		details = fields
	}
	attrs := []slog.Attr{
		slog.Any("response_error", buildResponseErrorLog(status, ae.Code, ae.Message, details)),
		slog.String("error_kind", ae.Kind.String()),
	}
	if cause := ae.Cause(); cause != nil {
		attrs = append(attrs,
			slog.String("source_error_type", fmt.Sprintf("%T", cause)),
			slog.String("source_error_message", sanitizeFreeTextLogValue(cause.Error())),
		)
	}
	if withStack && ae.Stack() != "" {
		attrs = append(attrs, slog.String("stack", ae.Stack()))
	}

	return attrs
}

type responseErrorLog struct {
	Status  int
	Code    string
	Message string
	Details any
}

func buildResponseErrorLog(status int, code, message string, details any) responseErrorLog {
	return responseErrorLog{Status: status, Code: code, Message: message, Details: sanitizeLogValue(details, "")}
}

// LogValue implements slog.LogValuer.
func (l responseErrorLog) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.Int("status", l.Status),
		slog.String("code", l.Code),
		slog.String("message", l.Message),
	}
	if l.Details != nil {
		attrs = append(attrs, slog.Any("details", l.Details))
	}

	return slog.GroupValue(attrs...)
}
