// Package observability carries request-scoped correlation data (request id,
// logger and request info) through context.Context.
package observability

import (
	"context"
	"log/slog"

	"server-template/internal/pkg/apperr"
)

type contextKey uint8

const (
	correlationIDKey contextKey = iota + 1
	loggerKey
	requestInfoKey
)

// HeaderXRequestID is the HTTP header used to carry the correlation ID.
const HeaderXRequestID = "X-Request-Id"

// RequestInfo is the request-scoped record shared by middlewares. A pointer
// is stored in the context once by the request-id middleware; later stages
// fill it in so they need no string keys on the framework context.
type RequestInfo struct {
	// Err is the final application error rendered to the client, if any.
	Err *apperr.Error
	// Body is the bounded, still unsanitized copy of the JSON request body.
	Body []byte
}

// WithCorrelationID returns a context carrying the request correlation ID.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// CorrelationIDFromContext extracts the request correlation ID from context.
func CorrelationIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(correlationIDKey).(string)

	return id
}

// WithLogger returns a context carrying the request-scoped logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// LoggerFromContext extracts the request-scoped logger from context.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	logger, _ := ctx.Value(loggerKey).(*slog.Logger)

	return logger
}

// LoggerFromContextOrDefault returns the request-scoped logger or fallback.
func LoggerFromContextOrDefault(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if logger := LoggerFromContext(ctx); logger != nil {
		return logger
	}

	return fallback
}

// WithRequestInfo returns a context carrying info.
func WithRequestInfo(ctx context.Context, info *RequestInfo) context.Context {
	return context.WithValue(ctx, requestInfoKey, info)
}

// RequestInfoFrom returns the request info stored in ctx, or nil.
func RequestInfoFrom(ctx context.Context) *RequestInfo {
	info, _ := ctx.Value(requestInfoKey).(*RequestInfo)

	return info
}
