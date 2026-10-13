package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCORSAllowedOrigins(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "empty disables CORS", raw: "", want: nil},
		{
			name: "trims and deduplicates origins",
			raw:  " https://app.example.com,https://APP.EXAMPLE.COM, http://localhost:3000 ",
			want: []string{"https://app.example.com", "http://localhost:3000"},
		},
		{name: "normalizes scheme case", raw: "HTTPS://EXAMPLE.COM", want: []string{"https://example.com"}},
		{name: "rejects wildcard", raw: "*", wantErr: true},
		{name: "rejects path", raw: "https://app.example.com/path", wantErr: true},
		{name: "rejects unsupported scheme", raw: "ftp://app.example.com", wantErr: true},
		{name: "rejects empty list item", raw: "https://app.example.com,", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCORSAllowedOrigins(tt.raw)
			if tt.wantErr {
				require.Error(t, err)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewCORS_UsesExplicitOrigins(t *testing.T) {
	mw, err := NewCORS(CORSOptions{AllowedOrigins: "https://app.example.com"})
	require.NoError(t, err)
	require.NotNil(t, mw)

	e := echo.New()
	e.Use(mw)
	e.GET("/resource", func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })

	do := func(method, origin string, extra map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), method, "/resource", nil)
		req.Header.Set(echo.HeaderOrigin, origin)
		for k, v := range extra {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		return rec
	}

	allowed := do(http.MethodGet, "https://app.example.com", nil)
	assert.Equal(t, "https://app.example.com", allowed.Header().Get(echo.HeaderAccessControlAllowOrigin))

	denied := do(http.MethodGet, "https://evil.example.com", nil)
	assert.Empty(t, denied.Header().Get(echo.HeaderAccessControlAllowOrigin))

	preflight := do(http.MethodOptions, "https://app.example.com", map[string]string{
		echo.HeaderAccessControlRequestMethod:  http.MethodPost,
		echo.HeaderAccessControlRequestHeaders: echo.HeaderAuthorization,
	})
	assert.Equal(t, http.StatusNoContent, preflight.Code)
	assert.Contains(t, preflight.Header().Get(echo.HeaderAccessControlAllowHeaders), echo.HeaderAuthorization)
	assert.Empty(t, preflight.Header().Get(echo.HeaderAccessControlAllowCredentials))
}

func TestNewCORS_EmptyIsDisabled(t *testing.T) {
	mw, err := NewCORS(CORSOptions{})
	require.NoError(t, err)
	assert.Nil(t, mw)
}
