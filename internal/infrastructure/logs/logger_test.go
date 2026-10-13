package logs

import (
	"log/slog"
	"testing"

	"server-template/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{}
	cfg.Env.Log.Level = "debug"

	called := false
	logger, err := New(Params{Config: cfg, WrapHandler: func(h slog.Handler) slog.Handler {
		called = true

		return h
	}})
	require.NoError(t, err)
	assert.True(t, called)
	assert.Same(t, logger.Handler(), slog.Default().Handler())
}

func TestNew_BadLevel(t *testing.T) {
	cfg := &config.Config{}
	cfg.Env.Log.Level = "loud"

	_, err := New(Params{Config: cfg})
	require.Error(t, err)
}
