// Package http2 serves the shared Echo handler over HTTP/1.1 and cleartext
// HTTP/2 (h2c) on one TCP port. TLS is expected to terminate at a proxy.
package http2

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"server-template/config"
	"server-template/internal/domain/delivery"
	"server-template/internal/domain/lifecycle"

	"github.com/labstack/echo/v5"
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
	http   *http.Server
}

// New returns the HTTP/1.1 + h2c delivery and registers its graceful shutdown
// on the fx lifecycle. Tag it `group:"deliveries"` when wiring.
func New(p Params) delivery.Delivery {
	s := &server{logger: p.Logger, http: newHTTPServer(p.Config, p.Echo)}

	p.Lifecycle.Append(fx.Hook{OnStop: s.stop})

	return s
}

// newHTTPServer builds the net/http server: HTTP/1 plus unencrypted HTTP/2
// served by the standard library (no golang.org/x/net/http2 needed).
func newHTTPServer(cfg *config.Config, handler http.Handler) *http.Server {
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:           handler,
		Protocols:         &protocols,
		ReadTimeout:       cfg.HTTP.Timeouts.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.Timeouts.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.Timeouts.WriteTimeout,
		IdleTimeout:       cfg.HTTP.Timeouts.IdleTimeout,
	}
	if cfg.HTTP.MaxHeaderValueCount > 0 {
		srv.MaxHeaderValueCount = cfg.HTTP.MaxHeaderValueCount
	}

	return srv
}

// Serve blocks until the server is shut down. A clean shutdown returns nil.
func (s *server) Serve(ctx context.Context) error {
	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", s.http.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.http.Addr, err)
	}

	return s.serve(ln)
}

func (s *server) serve(ln net.Listener) error {
	s.logger.Info("Starting HTTP server (HTTP/1.1 + h2c)", slog.String("addr", ln.Addr().String()))
	if err := s.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http: %w", err)
	}

	return nil
}

func (s *server) stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, lifecycle.DefaultTimeout)
	defer cancel()

	s.logger.Info("Shutting down HTTP server")
	if err := s.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	return nil
}
