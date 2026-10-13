package middleware

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"server-template/internal/delivery/http/response"
	"server-template/internal/pkg/apperr"
	"server-template/internal/pkg/observability"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestEcho builds the production-shaped chain around the given routes.
func newTestEcho(t *testing.T, opts RequestLoggerOptions) (*echo.Echo, *bytes.Buffer) {
	t.Helper()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	e := echo.New()
	e.JSONSerializer = response.NewJSONSerializer()
	e.HTTPErrorHandler = HTTPErrorHandler
	e.Use(RequestID(logger), RequestLogger(logger, opts), HandleErrors(), CaptureRequestBody())

	return e, &logs
}

func serve(t *testing.T, e *echo.Echo, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	return rec
}

func TestHandleErrors(t *testing.T) {
	userNotFound := apperr.New(apperr.NotFound, "USER_NOT_FOUND", "user not found")
	tests := []struct {
		name       string
		method     string
		path       string
		err        error
		wantStatus int
		wantCode   string
		notWant    string
	}{
		{name: "apperr", method: "GET", path: "/boom", err: apperr.Wrap(errors.New("sql: no rows"), userNotFound),
			wantStatus: 404, wantCode: "USER_NOT_FOUND", notWant: "sql"},
		{name: "plain error is generic 500", method: "GET", path: "/boom", err: errors.New("dial tcp 10.1.1.1: refused"),
			wantStatus: 500, wantCode: "INTERNAL_ERROR", notWant: "10.1.1.1"},
		{name: "echo http error", method: "GET", path: "/boom", err: echo.NewHTTPError(http.StatusForbidden, "nope"),
			wantStatus: 403, wantCode: "FORBIDDEN"},
		{name: "unknown route", method: "GET", path: "/missing", wantStatus: 404, wantCode: "NOT_FOUND"},
		{name: "method not allowed", method: "POST", path: "/boom", wantStatus: 405, wantCode: "METHOD_NOT_ALLOWED"},
		{name: "payload too large", method: "GET", path: "/boom", err: echo.NewHTTPError(http.StatusRequestEntityTooLarge, "big"),
			wantStatus: 413, wantCode: "PAYLOAD_TOO_LARGE"},
		{name: "unsupported media type", method: "GET", path: "/boom", err: echo.NewHTTPError(http.StatusUnsupportedMediaType, "mt"),
			wantStatus: 415, wantCode: "UNSUPPORTED_MEDIA_TYPE"},
		{name: "request timeout", method: "GET", path: "/boom", err: echo.NewHTTPError(http.StatusRequestTimeout, "slow"),
			wantStatus: 408, wantCode: "REQUEST_TIMEOUT"},
		{name: "apperr ignores wrapped echo status", method: "GET", path: "/boom",
			err:        apperr.Wrap(echo.NewHTTPError(http.StatusRequestEntityTooLarge, "x"), userNotFound),
			wantStatus: 404, wantCode: "USER_NOT_FOUND"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEcho(t, RequestLoggerOptions{})
			e.GET("/boom", func(*echo.Context) error { return tt.err })

			rec := serve(t, e, tt.method, tt.path, "")

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Contains(t, rec.Body.String(), `"code":"`+tt.wantCode+`"`)
			assert.Contains(t, rec.Body.String(), `"request_id":"`)
			assert.NotEmpty(t, rec.Header().Get(observability.HeaderXRequestID))
			if tt.notWant != "" {
				assert.NotContains(t, rec.Body.String(), tt.notWant)
			}
		})
	}
}

func TestHandleErrors_RecordsErrorInRequestInfo(t *testing.T) {
	var got *apperr.Error
	// Observe RequestInfo from an outer middleware after the chain finished.
	outer := echo.New()
	outer.JSONSerializer = response.NewJSONSerializer()
	outer.Use(RequestID(slog.Default()), func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			err := next(c)
			got = observability.RequestInfoFrom(c.Request().Context()).Err

			return err
		}
	}, HandleErrors())
	outer.GET("/x", func(*echo.Context) error { return apperr.ErrConflict })

	rec := serve(t, outer, "GET", "/x", "")

	assert.Equal(t, http.StatusConflict, rec.Code)
	require.NotNil(t, got)
	assert.Equal(t, "CONFLICT", got.Code)
}

func TestHandleErrors_CommittedResponseIsNotRewritten(t *testing.T) {
	e, _ := newTestEcho(t, RequestLoggerOptions{})
	e.GET("/x", func(c *echo.Context) error {
		_ = c.String(http.StatusAccepted, "partial")

		return errors.New("late failure")
	})

	rec := serve(t, e, "GET", "/x", "")

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "partial", rec.Body.String())
}

func TestHTTPErrorHandler_RendersEnvelope(t *testing.T) {
	e := echo.New()
	e.JSONSerializer = response.NewJSONSerializer()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), rec)

	HTTPErrorHandler(c, echo.ErrNotFound)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"NOT_FOUND"`)
}
