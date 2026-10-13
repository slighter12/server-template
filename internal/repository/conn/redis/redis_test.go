package redis

import (
	"context"
	"testing"
	"time"

	"server-template/config"

	redisLib "github.com/slighter12/go-lib/database/redis/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func TestNew_EmptyConfigIsError(t *testing.T) {
	for name, cfg := range map[string]*config.Config{
		"missing section": {},
		"no address":      {Redis: &redisLib.Conn{}},
	} {
		t.Run(name, func(t *testing.T) {
			client, err := New(Params{Lifecycle: fxtest.NewLifecycle(t), Config: cfg})
			require.Error(t, err)
			assert.Nil(t, client)
			assert.Contains(t, err.Error(), "redis")
		})
	}
}

func TestNew_UnreachableFailsOnStart(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	cfg := &config.Config{Redis: &redisLib.Conn{
		Address:     []string{"127.0.0.1:1"},
		DialTimeout: 200 * time.Millisecond,
	}}

	client, err := New(Params{Lifecycle: lc, Config: cfg})
	require.NoError(t, err)
	require.NotNil(t, client)
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	err = lc.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to ping Redis")
}
