package middleware

import (
	"fmt"

	"github.com/labstack/echo/v5"
)

// AltSvc advertises the HTTP/3 endpoint on UDP port in every response of the
// HTTP/1.1 and HTTP/2 server, so clients can upgrade to h3. Install it only
// when HTTP/3 is enabled.
func AltSvc(port int) echo.MiddlewareFunc {
	value := fmt.Sprintf(`h3=":%d"; ma=86400`, port)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Response().Header().Set("Alt-Svc", value)

			return next(c)
		}
	}
}
