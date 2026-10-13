package mysql

import (
	"context"
	"testing"
	"time"

	"server-template/config"

	mysqlLib "github.com/slighter12/go-lib/database/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func TestNew_EmptyConfigIsError(t *testing.T) {
	_, err := New(Params{Lifecycle: fxtest.NewLifecycle(t), Config: &config.Config{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mysql")
}

func TestNew_UnreachableFailsOnStart(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	cfg := &config.Config{Mysql: map[string]*mysqlLib.DBConn{"main": {
		Database: "app",
		Master: mysqlLib.ConnectionConfig{
			Host: "127.0.0.1", Port: "1", UserName: "app", Password: "app", Timeout: 200 * time.Millisecond,
		},
	}}}

	result, err := New(Params{Lifecycle: lc, Config: cfg})
	require.NoError(t, err)
	require.NotNil(t, result.Default)

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	err = lc.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ping MySQL")
}
