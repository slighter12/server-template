package http3

import (
	"io"
	"log/slog"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"server-template/config"
)

func TestDisabledIsNoop(t *testing.T) {
	d, err := New(Params{
		Lifecycle: fx.Lifecycle(nil),
		Config:    &config.Config{},
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Echo:      echo.New(),
	})

	require.NoError(t, err)
	assert.NoError(t, d.Serve(t.Context()))
}

func TestEnabledRegistersShutdownHook(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP3.Enable = true
	cfg.HTTP3.Port = 8443
	lc := fxtest.NewLifecycle(t)

	d, err := New(Params{Lifecycle: lc, Config: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Echo: echo.New()})

	require.NoError(t, err)
	assert.IsType(t, &server{}, d)
	require.NoError(t, lc.Start(t.Context()))
	require.NoError(t, lc.Stop(t.Context()))
}
