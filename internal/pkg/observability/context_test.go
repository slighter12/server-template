package observability_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"server-template/internal/pkg/observability"
)

func TestContextValues(t *testing.T) {
	ctx := t.Context()
	assert.Empty(t, observability.CorrelationIDFromContext(ctx))
	assert.Nil(t, observability.LoggerFromContext(ctx))
	assert.Nil(t, observability.RequestInfoFrom(ctx))

	fallback := slog.Default()
	assert.Same(t, fallback, observability.LoggerFromContextOrDefault(ctx, fallback))

	logger := slog.New(slog.DiscardHandler)
	info := &observability.RequestInfo{}
	ctx = observability.WithCorrelationID(ctx, "rid")
	ctx = observability.WithLogger(ctx, logger)
	ctx = observability.WithRequestInfo(ctx, info)

	assert.Equal(t, "rid", observability.CorrelationIDFromContext(ctx))
	assert.Same(t, logger, observability.LoggerFromContext(ctx))
	assert.Same(t, info, observability.RequestInfoFrom(ctx))
}
