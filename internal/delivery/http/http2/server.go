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
	"server-template/internal/domain/usecase"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

type HTTP2Params struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Logger    *slog.Logger
	AuthUC    usecase.AuthHTTPUseCase
}

type http2Server struct {
	cfg    *config.Config
	logger *slog.Logger
	server *http.Server
}

func NewHTTP2(params HTTP2Params) (delivery.Delivery, error) {
	echoServer := echo.New()
	router.RegisterRoutes(router.RouterParams{
		Router: echoServer,
		Config: params.Config,
		Logger: params.Logger,
		AuthUC: params.AuthUC,
	})

	certificates, err := common.GenerateTLSConfig()
	if err != nil {
		return nil, errors.Wrap(err, "generate TLS config")
	}

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", params.Config.HTTP.Port),
		Handler:           echoServer,
		ReadHeaderTimeout: 20 * time.Second,
		TLSConfig: &tls.Config{
			Certificates: certificates,
			MinVersion:   tls.VersionTLS12,
			NextProtos:   []string{"h2"},
		},
	}

	delivery := &http2Server{
		cfg:    params.Config,
		logger: params.Logger,
		server: server,
	}

	params.Lifecycle.Append(fx.Hook{
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
