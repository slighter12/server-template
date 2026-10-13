package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"server-template/internal/pkg/apperr"
)

func lastLog(t *testing.T, raw string) map[string]any {
	t.Helper()

	lines := splitLines(raw)
	require.NotEmpty(t, lines, "expected a log line")
	var rec map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &rec))

	return rec
}

func splitLines(s string) []string {
	var out []string
	for line := range stringsSplitSeq(s) {
		if line != "" {
			out = append(out, line)
		}
	}

	return out
}

func TestRequestLogger_Levels(t *testing.T) {
	tests := []struct {
		name      string
		debug     bool
		err       error
		wantLog   bool
		wantLevel string
		wantStack bool
	}{
		{name: "success is silent", wantLog: false},
		{name: "success in debug", debug: true, wantLog: true, wantLevel: "INFO"},
		{name: "client error warns without stack", err: apperr.ErrNotFound, wantLog: true, wantLevel: "WARN"},
		{name: "server fault errors with stack", err: apperr.Wrap(errors.New("db down"), apperr.ErrInternal), wantLog: true, wantLevel: "ERROR", wantStack: true},
		{name: "plain error is a server fault", err: errors.New("boom"), wantLog: true, wantLevel: "ERROR", wantStack: true},
		{name: "unavailable errors", err: apperr.ErrUnavailable, wantLog: true, wantLevel: "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, logs := newTestEcho(t, RequestLoggerOptions{Debug: tt.debug})
			e.GET("/x", func(c *echo.Context) error {
				if tt.err != nil {
					return tt.err
				}

				return c.NoContent(http.StatusNoContent)
			})

			serve(t, e, http.MethodGet, "/x", "")

			if !tt.wantLog {
				assert.Empty(t, logs.String())

				return
			}
			rec := lastLog(t, logs.String())
			assert.Equal(t, tt.wantLevel, rec["level"])
			assert.NotEmpty(t, rec["request_id"])
			_, hasStack := rec["stack"]
			assert.Equal(t, tt.wantStack, hasStack)
		})
	}
}

func TestRequestLogger_SanitizesBodyAndCause(t *testing.T) {
	e, logs := newTestEcho(t, RequestLoggerOptions{})
	e.POST("/x", func(c *echo.Context) error {
		return apperr.Wrap(errors.New("lookup failed for user@example.com token=abc123"), apperr.ErrInternal)
	})

	serve(t, e, http.MethodPost, "/x?access_token=zzz", `{"name":"n","password":"hunter2","email":"u@e.com"}`)

	out := logs.String()
	assert.NotContains(t, out, "hunter2")
	assert.NotContains(t, out, "u@e.com")
	assert.NotContains(t, out, "user@example.com")
	assert.NotContains(t, out, "abc123")
	assert.NotContains(t, out, "zzz")
	assert.Contains(t, out, `"name":"n"`)
	assert.Contains(t, out, "[REDACTED]")
	rec := lastLog(t, out)
	assert.Contains(t, rec["source_error_message"], "[REDACTED]")
}

func TestRequestLogger_ValidationFieldsLoggedForClientErrors(t *testing.T) {
	e, logs := newTestEcho(t, RequestLoggerOptions{})
	e.GET("/x", func(*echo.Context) error {
		return apperr.ErrValidationFailed.WithFields(apperr.FieldViolation{Field: "name", Description: "name is required"})
	})

	serve(t, e, http.MethodGet, "/x", "")

	assert.Contains(t, logs.String(), "name is required")
	assert.Contains(t, logs.String(), `"code":"VALIDATION_FAILED"`)
}

func TestRequestLogger_TraceAttributes(t *testing.T) {
	e, logs := newTestEcho(t, RequestLoggerOptions{Debug: true})
	e.GET("/x", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
	})
	req := newRequestWithSpan(t, sc)

	e.ServeHTTP(newRecorder(), req)

	rec := lastLog(t, logs.String())
	assert.Equal(t, sc.TraceID().String(), rec["trace_id"])
	assert.Equal(t, sc.SpanID().String(), rec["span_id"])
}

func TestRequestLogger_NoTraceAttributesWithoutSpan(t *testing.T) {
	e, logs := newTestEcho(t, RequestLoggerOptions{Debug: true})
	e.GET("/x", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })

	serve(t, e, http.MethodGet, "/x", "")

	rec := lastLog(t, logs.String())
	assert.NotContains(t, rec, "trace_id")
}
