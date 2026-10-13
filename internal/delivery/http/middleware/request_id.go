package middleware

import (
	"log/slog"
	"uuid"

	"github.com/labstack/echo/v5"

	"server-template/internal/pkg/observability"
)

const maxRequestIDLen = 128

// RequestID reuses a well-formed inbound X-Request-Id or generates a UUIDv7,
// echoes it in the response, and stores the id, a child logger and a fresh
// *observability.RequestInfo in the request context.
func RequestID(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			requestID := c.Request().Header.Get(observability.HeaderXRequestID)
			if !validRequestID(requestID) {
				requestID = uuid.NewV7().String()
			}
			c.Response().Header().Set(observability.HeaderXRequestID, requestID)

			ctx := c.Request().Context()
			ctx = observability.WithCorrelationID(ctx, requestID)
			ctx = observability.WithLogger(ctx, logger.With(slog.String("request_id", requestID)))
			ctx = observability.WithRequestInfo(ctx, &observability.RequestInfo{})
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

// validRequestID accepts short ids made of safe characters only, so a client
// cannot inject header or log content.
func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for i := range len(id) {
		ch := id[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9':
		case ch == '-', ch == '_', ch == '.', ch == ':':
		default:
			return false
		}
	}

	return true
}
