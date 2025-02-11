package http

import (
	"context"
	"fmt"
	"net/http"

	"server-template/config"
	"server-template/internal/domain/delivery"

	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

type httpServer struct {
	fx.In
	cfg *config.Config
}

func NewHTTP(cfg *config.Config) delivery.Delivery {
	return &httpServer{
		cfg: cfg,
	}
}

func (s *httpServer) Serve(lc fx.Lifecycle, ctx context.Context) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			e := echo.New()
			registerRoutes(e)

			return e.Start(fmt.Sprintf(":%d", s.cfg.HTTP.Port))
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
}

func registerRoutes(r *echo.Echo) {
	r.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "pong"})
	})
}
