// Package http3 serves the shared Echo handler over HTTP/3 (QUIC, UDP).
// It is disabled by default; see config HTTP3.Enable.
package http3

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"server-template/config"
	"server-template/internal/delivery/http/common"
	"server-template/internal/domain/delivery"
	"server-template/internal/domain/lifecycle"

	"github.com/labstack/echo/v5"
	"github.com/quic-go/quic-go"
	quichttp3 "github.com/quic-go/quic-go/http3"
	"go.uber.org/fx"
)

// Params are the dependencies of New.
type Params struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Logger    *slog.Logger
	Echo      *echo.Echo
}

type server struct {
	logger *slog.Logger
	port   int
	h3     *quichttp3.Server
}

// noop is the Delivery used when HTTP/3 is disabled.
type noop struct{}

func (noop) Serve(context.Context) error { return nil }

// New returns the HTTP/3 delivery, or a no-op Delivery whose Serve returns nil
// immediately when HTTP3.Enable is false. Tag it `group:"deliveries"` when
// wiring. The AltSvc middleware advertising this port is installed by the
// server package, not here.
func New(p Params) (delivery.Delivery, error) {
	if !p.Config.HTTP3.Enable {
		return noop{}, nil
	}

	certs, err := common.GenerateTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("generate TLS config: %w", err)
	}

	s := &server{
		logger: p.Logger,
		port:   p.Config.HTTP3.Port,
		h3: &quichttp3.Server{
			Port:        p.Config.HTTP3.Port,
			Handler:     p.Echo,
			IdleTimeout: p.Config.HTTP.Timeouts.IdleTimeout,
			TLSConfig: &tls.Config{
				Certificates: certs,
				MinVersion:   tls.VersionTLS13,
			},
			QUICConfig: &quic.Config{
				MaxIdleTimeout:  30 * time.Second,
				KeepAlivePeriod: 10 * time.Second,
			},
		},
	}
	p.Lifecycle.Append(fx.Hook{OnStop: s.stop})

	return s, nil
}

// Serve blocks until the server is shut down. A clean shutdown returns nil.
func (s *server) Serve(ctx context.Context) error {
	lc := net.ListenConfig{}
	pc, err := lc.ListenPacket(ctx, "udp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("listen udp :%d: %w", s.port, err)
	}
	defer pc.Close()

	s.logger.Info("Starting HTTP/3 server", slog.Int("port", s.port))
	if err := s.h3.Serve(pc); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http3: %w", err)
	}

	return nil
}

func (s *server) stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, lifecycle.DefaultTimeout)
	defer cancel()

	s.logger.Info("Shutting down HTTP/3 server")
	if err := s.h3.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http3 server: %w", err)
	}

	return nil
}
