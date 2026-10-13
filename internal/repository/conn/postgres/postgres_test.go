package postgres

import (
	"context"
	"testing"
	"time"

	"server-template/config"

	pgLib "github.com/slighter12/go-lib/database/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func TestNew_EmptyConfigIsError(t *testing.T) {
	_, err := New(Params{Lifecycle: fxtest.NewLifecycle(t), Config: &config.Config{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "postgres")
}

func TestNew_UnreachableFailsOnStart(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	cfg := &config.Config{Postgres: map[string]*pgLib.DBConn{"main": {
		Database: "app",
		SSLMode:  "disable",
		Master: pgLib.ConnectionConfig{
			Host: "127.0.0.1", Port: "1", UserName: "app", Password: "app",
		},
	}}}

	result, err := New(Params{Lifecycle: lc, Config: cfg})
	require.NoError(t, err)
	require.NotNil(t, result.Default)

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	err = lc.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ping PostgreSQL")
}
