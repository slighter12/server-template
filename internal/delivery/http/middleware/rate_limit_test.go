package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimiter_Disabled(t *testing.T) {
	mw, err := NewRateLimiter(RateLimitOptions{}, nil)

	require.NoError(t, err)
	assert.Nil(t, mw)
}

func TestNewRateLimiter_InvalidOptions(t *testing.T) {
	_, err := NewRateLimiter(RateLimitOptions{Rate: 1, Burst: 0, ExpiresIn: time.Minute}, nil)
	require.Error(t, err)

	_, err = NewRateLimiter(RateLimitOptions{Rate: 1, Burst: 1}, nil)
	require.Error(t, err)
}

func TestNewRateLimiter_DeniesWithTooManyRequests(t *testing.T) {
	mw, err := NewRateLimiter(RateLimitOptions{Rate: 0.001, Burst: 2, ExpiresIn: time.Minute},
		func(c *echo.Context) string { return c.Request().Header.Get("X-Key") })
	require.NoError(t, err)

	e, _ := newTestEcho(t, RequestLoggerOptions{})
	e.GET("/", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) }, mw)

	do := func(key string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		req.Header.Set("X-Key", key)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		return rec
	}

	assert.Equal(t, http.StatusNoContent, do("a").Code)
	assert.Equal(t, http.StatusNoContent, do("a").Code)
	denied := do("a")
	assert.Equal(t, http.StatusTooManyRequests, denied.Code)
	assert.Contains(t, denied.Body.String(), `"code":"TOO_MANY_REQUESTS"`)
	assert.Equal(t, http.StatusNoContent, do("b").Code, "other keys have their own bucket")
}

func TestKeyByIP(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.7:1234"

	assert.Equal(t, "192.0.2.7", KeyByIP(e.NewContext(req, httptest.NewRecorder())))
}
