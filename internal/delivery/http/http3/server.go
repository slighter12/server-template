package http3

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"server-template/config"
	"server-template/internal/delivery/http/common"
	"server-template/internal/delivery/http/router"
	"server-template/internal/domain/delivery"
	"server-template/internal/domain/lifecycle"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"go.uber.org/fx"
)

type http3Server struct {
	cfg    *config.Config
	logger *slog.Logger
	server *http3.Server
}

func NewHTTP3(lc fx.Lifecycle, cfg *config.Config, logger *slog.Logger) (delivery.Delivery, error) {
	echoServer := echo.New()
	router.RegisterRoutes(echoServer, cfg, logger)

	certificates, err := common.GenerateTLSConfig()
	if err != nil {
		return nil, errors.Wrap(err, "generate TLS config")
	}

	server := &http3.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler: echoServer,
		TLSConfig: &tls.Config{
			Certificates: certificates,
			MinVersion:   tls.VersionTLS12,
			NextProtos:   []string{"h3", "h3-29"},
		},
		QUICConfig: &quic.Config{
			MaxIdleTimeout:             30 * time.Second,
			KeepAlivePeriod:            10 * time.Second,
			EnableDatagrams:            true,
			MaxIncomingStreams:         1000,
			MaxStreamReceiveWindow:     10 * 1024 * 1024,
			MaxConnectionReceiveWindow: 15 * 1024 * 1024,
			Allow0RTT:                  true,
		},
	}

	delivery := &http3Server{
		cfg:    cfg,
		logger: logger,
		server: server,
	}

	lc.Append(fx.Hook{
		OnStop: delivery.stop,
	})

	return delivery, nil
}

func (s *http3Server) Serve(ctx context.Context) error {
	udpAddr := &net.UDPAddr{Port: s.cfg.HTTP.Port}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return errors.Wrap(err, "failed to create UDP listener")
	}

	s.logger.Info("Starting HTTP/3 server", slog.Any("port", s.cfg.HTTP.Port))
	if err := s.server.Serve(udpConn); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return errors.Wrap(err, "failed to serve http3")
	}

	return nil
}

func (s *http3Server) stop(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP/3 server")

	shutdownCtx, cancel := context.WithTimeout(ctx, lifecycle.DefaultTimeout)
	defer cancel()

	err := s.server.Shutdown(shutdownCtx)
	if err != nil {
		s.logger.Error("Failed to close HTTP/3 server", slog.Any("error", err))

		return errors.WithStack(err)
	}

	return nil
}
