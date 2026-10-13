package mongo

import (
	"context"
	"testing"
	"time"

	"server-template/config"

	mongoLib "github.com/slighter12/go-lib/database/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func TestNew_EmptyConfigIsError(t *testing.T) {
	_, err := New(Params{Lifecycle: fxtest.NewLifecycle(t), Config: &config.Config{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mongo")
}

func TestNew_UnreachableFailsOnStart(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	cfg := &config.Config{Mongo: map[string]*mongoLib.DBConn{"main": {
		Hosts:          []string{"127.0.0.1:1"},
		ConnectTimeout: 200 * time.Millisecond,
		Options:        map[string]string{"serverSelectionTimeoutMS": "500"},
	}}}

	result, err := New(Params{Lifecycle: lc, Config: cfg})
	require.NoError(t, err)
	require.NotNil(t, result.Default)

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	err = lc.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ping MongoDB")
}
