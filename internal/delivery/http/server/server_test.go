package server_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"server-template/config"
	"server-template/internal/delivery/http/response"
	"server-template/internal/delivery/http/server"
	"server-template/internal/pkg/apperr"
	"server-template/internal/pkg/observability"
)

type testRoutes struct{}

var errTeapot = apperr.New(apperr.Conflict, "TEAPOT", "teapot conflict")

func (testRoutes) Register(e *echo.Echo) {
	e.GET("/fail", func(*echo.Context) error { return errTeapot })
	e.POST("/echo", func(c *echo.Context) error {
		var in struct {
			Name string `json:"name" validate:"required"`
		}
		if err := c.Bind(&in); err != nil {
			return err
		}

		return response.Success(c, http.StatusOK, in)
	})
}

func newConfig() *config.Config {
	cfg := &config.Config{}
	cfg.HTTP.MaxRequestBodySize = "1K"

	return cfg
}

func build(t *testing.T, cfg *config.Config) *echo.Echo {
	t.Helper()

	e, err := server.New(server.Params{
		Config: cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Routes: []server.RouteRegistrar{testRoutes{}},
	})
	require.NoError(t, err)

	return e
}

func do(t *testing.T, e *echo.Echo, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	return rec
}

func TestHealth(t *testing.T) {
	rec := do(t, build(t, newConfig()), http.MethodGet, "/health", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
	assert.NotEmpty(t, rec.Header().Get(observability.HeaderXRequestID))
	assert.Empty(t, rec.Header().Get("Traceparent"))
	assert.Empty(t, rec.Header().Get("Alt-Svc"))
}

func TestErrorEnvelopeAndRequestID(t *testing.T) {
	rec := do(t, build(t, newConfig()), http.MethodGet, "/fail", "")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"TEAPOT"`)
	assert.Contains(t, rec.Body.String(), `"request_id":"`+rec.Header().Get(observability.HeaderXRequestID)+`"`)
}

func TestTransportStatusFidelity(t *testing.T) {
	e := build(t, newConfig())

	assert.Equal(t, http.StatusNotFound, do(t, e, http.MethodGet, "/missing", "").Code)
	assert.Equal(t, http.StatusMethodNotAllowed, do(t, e, http.MethodDelete, "/health", "").Code)

	big := `{"name":"` + strings.Repeat("x", 2048) + `"}`
	rec := do(t, e, http.MethodPost, "/echo", big)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"PAYLOAD_TOO_LARGE"`)
}

func TestAltSvcOnlyWhenHTTP3Enabled(t *testing.T) {
	cfg := newConfig()
	cfg.HTTP3.Enable = true
	cfg.HTTP3.Port = 8443

	rec := do(t, build(t, cfg), http.MethodGet, "/health", "")

	assert.Contains(t, rec.Header().Get("Alt-Svc"), `h3=":8443"`)
}

func TestRateLimit(t *testing.T) {
	cfg := newConfig()
	cfg.HTTP.RateLimit.Enable = true
	cfg.HTTP.RateLimit.RequestsPerSecond = 0.001
	cfg.HTTP.RateLimit.Burst = 1
	e := build(t, cfg)

	assert.Equal(t, http.StatusOK, do(t, e, http.MethodGet, "/health", "").Code)
	assert.Equal(t, http.StatusTooManyRequests, do(t, e, http.MethodGet, "/health", "").Code)
}

func TestTraceparentWhenOTelEnabled(t *testing.T) {
	previous := otel.GetTracerProvider()
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = tp.Shutdown(t.Context())
	})

	cfg := newConfig()
	cfg.Observability.Otel.Enable = true
	cfg.Env.ServiceName = "server-template-test"

	rec := do(t, build(t, cfg), http.MethodGet, "/health", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Regexp(t, `^00-[0-9a-f]{32}-[0-9a-f]{16}-0[01]$`, rec.Header().Get("Traceparent"))
}

func TestInvalidConfig(t *testing.T) {
	cfg := newConfig()
	cfg.HTTP.MaxRequestBodySize = "lots"

	_, err := server.New(server.Params{Config: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	require.Error(t, err)
}
