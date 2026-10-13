package middleware

import (
	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel/propagation"
)

// Traceparent writes the W3C `traceparent` (and `tracestate`) response header
// from the span active in the request context, letting clients and support
// staff correlate a response with its trace. Install it after the OTel
// middleware; without an active span it writes nothing.
func Traceparent() echo.MiddlewareFunc {
	var propagator propagation.TraceContext

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			propagator.Inject(c.Request().Context(), propagation.HeaderCarrier(c.Response().Header()))

			return next(c)
		}
	}
}
