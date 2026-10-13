package mysql

import (
	"testing"

	"server-template/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
)

func TestNew_EmptyConfigIsOptional(t *testing.T) {
	lc := fxtest.NewLifecycle(t)

	result, err := New(Params{Lifecycle: lc, Config: &config.Config{}})
	require.NoError(t, err)
	assert.Empty(t, result.Clients)
	assert.Nil(t, result.Default)
}
