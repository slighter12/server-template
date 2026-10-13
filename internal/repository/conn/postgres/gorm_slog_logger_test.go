package postgres

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"server-template/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"gorm.io/gorm/logger"
)

func TestGormSlogLoggerTrace_RedactsSQLLiteralValues(t *testing.T) {
	var logs bytes.Buffer
	loggerUnderTest := &gormSlogLogger{
		logger:        slog.New(slog.NewJSONHandler(&logs, nil)),
		level:         logger.Error,
		slowThreshold: defaultGormSlowThreshold,
	}

	loggerUnderTest.Trace(
		context.Background(),
		time.Now(),
		func() (string, int64) {
			return `INSERT INTO "users" ("name","email") VALUES ('dev','123123')`, 0
		},
		errors.New(`ERROR: duplicate key value violates unique constraint "idx_users_email_active"; Key (email)=('123123')`),
	)

	logOutput := logs.String()
	assert.Contains(t, logOutput, `"sql"`)
	assert.Contains(t, logOutput, "'?'")
	assert.NotContains(t, logOutput, "123123")
	assert.NotContains(t, logOutput, "dev")
}

func TestSanitizeSQLLogString_RedactsEscapedStringLiteral(t *testing.T) {
	got := sanitizeSQLLogString(`VALUES ('Bob''s Shop','A123')`)

	assert.True(t, strings.Contains(got, "'?'"))
	assert.NotContains(t, got, "Bob")
	assert.NotContains(t, got, "A123")
}

func TestNew_EmptyConfigIsOptional(t *testing.T) {
	lc := fxtest.NewLifecycle(t)

	result, err := New(Params{Lifecycle: lc, Config: &config.Config{}})
	require.NoError(t, err)
	assert.Empty(t, result.Clients)
	assert.Nil(t, result.Default)
}
