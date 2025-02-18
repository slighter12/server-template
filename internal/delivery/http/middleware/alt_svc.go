package middleware

import (
	"fmt"

	"github.com/labstack/echo/v4"
)

func AltSvc(port int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Alt-Svc", fmt.Sprintf(`h3=":%d"`, port))

			return next(c)
		}
	}
}
