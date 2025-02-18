package router

import (
	"log/slog"
	"net/http"

	"server-template/config"
	"server-template/internal/delivery/http/middleware"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	slogecho "github.com/samber/slog-echo"
)

func RegisterRoutes(router *echo.Echo, cfg *config.Config, logger *slog.Logger) {
	router.Use(middleware.AltSvc(cfg.HTTP.Port))
	router.Use(slogecho.New(logger))
	router.Use(echomiddleware.Recover())

	router.GET("/ping", handlePing)
	router.GET("/protocol", handleProtocol)
}

func handlePing(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"message": "pong"})
}

func handleProtocol(c echo.Context) error {
	proto := "unknown"
	switch c.Request().ProtoMajor {
	case 3:
		proto = "HTTP/3"
	case 2:
		proto = "HTTP/2"
	case 1:
		proto = "HTTP/1.1"
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"protocol": proto,
		"headers":  c.Request().Header,
	})
}
