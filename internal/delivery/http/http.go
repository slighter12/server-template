package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"server-template/config"
	"server-template/internal/domain/delivery"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	slogecho "github.com/samber/slog-echo"
	"go.uber.org/fx"
)

type httpServer struct {
	fx.In

	cfg    *config.Config
	logger *slog.Logger
}

func NewHTTP(cfg *config.Config, logger *slog.Logger) delivery.Delivery {
	return &httpServer{
		cfg:    cfg,
		logger: logger,
	}
}

func (s *httpServer) Serve(lc fx.Lifecycle, ctx context.Context) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			e := echo.New()
			registerRoutes(e, s.logger)

			return e.Start(fmt.Sprintf(":%d", s.cfg.HTTP.Port))
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
}

func registerRoutes(router *echo.Echo, logger *slog.Logger) {
	router.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "pong"})
	})

	router.Use(slogecho.New(logger))
	router.Use(middleware.Recover())
}
