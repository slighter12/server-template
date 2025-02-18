package http2

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"server-template/config"
	"server-template/internal/delivery/http/common"
	"server-template/internal/delivery/http/router"
	"server-template/internal/domain/delivery"
	"server-template/internal/domain/lifecycle"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

type http2Server struct {
	cfg    *config.Config
	logger *slog.Logger
	server *http.Server
}

func NewHTTP2(lc fx.Lifecycle, cfg *config.Config, logger *slog.Logger) (delivery.Delivery, error) {
	echoServer := echo.New()
	router.RegisterRoutes(echoServer, cfg, logger)

	certificates, err := common.GenerateTLSConfig()
	if err != nil {
		return nil, errors.Wrap(err, "generate TLS config")
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:           echoServer,
		ReadHeaderTimeout: 20 * time.Second,
		TLSConfig: &tls.Config{
			Certificates: certificates,
			MinVersion:   tls.VersionTLS12,
			NextProtos:   []string{"h2"},
		},
	}

	delivery := &http2Server{
		cfg:    cfg,
		logger: logger,
		server: server,
	}

	lc.Append(fx.Hook{
		OnStop: delivery.stop,
	})

	return delivery, nil
}

func (s *http2Server) Serve(ctx context.Context) error {
	s.logger.Info("Starting HTTP/2 server", slog.Any("port", s.cfg.HTTP.Port))
	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return errors.Wrap(err, "failed to serve https")
	}

	return nil
}

func (s *http2Server) stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, lifecycle.DefaultTimeout)
	defer cancel()

	s.logger.Info("Shutting down HTTP/2 server")

	return errors.WithStack(s.server.Shutdown(shutdownCtx))
}
