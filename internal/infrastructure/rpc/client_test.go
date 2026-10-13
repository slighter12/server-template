package rpc_test

import (
	"context"
	"testing"

	"server-template/config"
	"server-template/internal/infrastructure/rpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func TestNewEmptyConfigIsError(t *testing.T) {
	c, err := rpc.New(rpc.Params{Lifecycle: fxtest.NewLifecycle(t), Config: &config.Config{}})
	require.Error(t, err)
	assert.Nil(t, c)
	assert.Contains(t, err.Error(), "rpc")
}

func TestNewConfiguredClient(t *testing.T) {
	cfg := &config.Config{}
	cfg.RPC.Clients = map[string]config.RPCClientConfig{"auth": {Target: "127.0.0.1:1"}}

	lc := fxtest.NewLifecycle(t)
	c, err := rpc.New(rpc.Params{Lifecycle: lc, Config: cfg})
	require.NoError(t, err)

	_, err = c.GetClient(rpc.AuthClient)
	require.NoError(t, err)
	_, err = c.GetClient("missing")
	require.Error(t, err)

	require.NoError(t, lc.Start(context.Background()))
	require.NoError(t, lc.Stop(context.Background()))
}
