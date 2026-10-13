package middleware

import (
	"bytes"
	"fmt"
	"io"

	"github.com/labstack/echo/v5"

	"server-template/internal/pkg/observability"
)

// CaptureRequestBody keeps a copy of JSON request bodies in RequestInfo so the
// request logger can log a sanitized request after binding. Install it after
// BodyLimit: the whole body is buffered in memory.
func CaptureRequestBody() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			info := observability.RequestInfoFrom(req.Context())
			if info == nil || req.Body == nil || !requestMayHaveBody(req.Method) ||
				!isJSONContentType(req.Header.Get(echo.HeaderContentType)) {
				return next(c)
			}

			original := req.Body
			body, err := io.ReadAll(original)
			_ = original.Close()
			if err != nil {
				return fmt.Errorf("read request body for error log: %w", err)
			}
			req.Body = io.NopCloser(bytes.NewReader(body))
			info.Body = body

			return next(c)
		}
	}
}
