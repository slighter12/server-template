// Package debug provides an optional diagnostics server (pprof + flight recorder).
package debug

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime/trace"
	"time"

	"server-template/config"
	"server-template/internal/domain/delivery"
	"server-template/internal/domain/lifecycle"

	"go.uber.org/fx"
)

const (
	flightMinAge   = 10 * time.Second
	flightMaxBytes = 16 << 20 // 16 MiB
)

// Params are the dependencies of NewDebugServer.
type Params struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Logger    *slog.Logger
}

type noop struct{}

func (noop) Serve(context.Context) error { return nil }

type server struct {
	addr   string
	logger *slog.Logger
	http   *http.Server
	fr     *trace.FlightRecorder

	ready chan net.Addr // receives the bound address (buffered, for tests)
}

// NewDebugServer returns the debug delivery, or a no-op one when cfg.DebugServer.Enable is false.
func NewDebugServer(p Params) delivery.Delivery {
	if !p.Config.DebugServer.Enable {
		return noop{}
	}

	s := newServer(p.Config.DebugServer.Addr, p.Logger)
	p.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, lifecycle.DefaultTimeout)
			defer cancel()

			return s.shutdown(ctx)
		},
	})

	return s
}

func newServer(addr string, logger *slog.Logger) *server {
	if addr == "" {
		addr = "127.0.0.1:6060"
	}
	if logger == nil {
		logger = slog.Default()
	}

	s := &server{
		addr:   addr,
		logger: logger,
		fr:     trace.NewFlightRecorder(trace.FlightRecorderConfig{MinAge: flightMinAge, MaxBytes: flightMaxBytes}),
		ready:  make(chan net.Addr, 1),
	}
	s.http = &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

// handler uses a dedicated mux so nothing leaks via http.DefaultServeMux.
func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	// Index also serves every named runtime profile under /debug/pprof/<name>,
	// including the Go 1.27 "goroutineleak" profile (/debug/pprof/goroutineleak).
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.HandleFunc("GET /debug/flightrecorder", s.flightRecorder)

	return mux
}

func (s *server) flightRecorder(w http.ResponseWriter, _ *http.Request) {
	if !s.fr.Enabled() {
		http.Error(w, "flight recorder not running", http.StatusServiceUnavailable)

		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="flight-%s.trace"`, time.Now().UTC().Format("20060102T150405Z")))
	if _, err := s.fr.WriteTo(w); err != nil {
		s.logger.Error("write flight recorder", slog.Any("error", err))
	}
}

// Serve starts the flight recorder and the HTTP listener; it blocks until shutdown.
func (s *server) Serve(ctx context.Context) error {
	if err := s.fr.Start(); err != nil {
		return fmt.Errorf("start flight recorder: %w", err)
	}
	defer s.fr.Stop()

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("debug server listen: %w", err)
	}
	s.ready <- ln.Addr()

	s.logger.Info("Starting debug server", slog.String("addr", ln.Addr().String()))
	if err := s.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("debug server: %w", err)
	}

	return nil
}

func (s *server) shutdown(ctx context.Context) error {
	s.logger.Info("Stopping debug server")
	s.fr.Stop()

	if err := s.http.Shutdown(ctx); err != nil {
		return fmt.Errorf("debug server shutdown: %w", err)
	}

	return nil
}
