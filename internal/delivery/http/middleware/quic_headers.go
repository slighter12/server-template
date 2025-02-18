package middleware

import (
	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/quic-go/quic-go/http3"
)

func SetQUICHeaders(h3Server *http3.Server, logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().ProtoMajor < 3 {
				if err := h3Server.SetQUICHeaders(c.Response().Header()); err != nil {
					logger.Error("Failed to set QUIC headers", slog.Any("error", err))
				}
			}

			return next(c)
		}
	}
}
