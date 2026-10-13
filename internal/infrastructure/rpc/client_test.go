package rpc_test

import (
	"context"
	"testing"

	"server-template/config"
	"server-template/internal/infrastructure/rpc"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func TestNewEmptyConfig(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	c, err := rpc.New(rpc.Params{Lifecycle: lc, Config: &config.Config{}})
	require.NoError(t, err)
	_, err = c.GetClient(rpc.AuthClient)
	require.Error(t, err)
	require.NoError(t, lc.Start(context.Background()))
	require.NoError(t, lc.Stop(context.Background()))
}
