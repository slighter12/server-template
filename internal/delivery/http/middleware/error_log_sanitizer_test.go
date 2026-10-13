package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSensitiveKey(t *testing.T) {
	for _, key := range []string{"password", "Authorization", "access-token", "api_token", "email", "phone", "address", "cookie", "secret"} {
		assert.True(t, isSensitiveKey(key), key)
	}
	for _, key := range []string{"name", "page", "lat", "store_name", "business_license"} {
		assert.False(t, isSensitiveKey(key), key)
	}
}

func TestSanitizeLogValue(t *testing.T) {
	got := sanitizeLogValue(map[string]any{
		"name":  "n",
		"token": "t",
		"nested": []any{
			map[string]any{"password": "p", "ok": "fine"},
		},
		"long": strings.Repeat("x", maxLoggedStringLength+10),
	}, "")

	m := got.(map[string]any)
	assert.Equal(t, "n", m["name"])
	assert.Equal(t, redactedLogValue, m["token"])
	assert.Equal(t, map[string]any{"password": redactedLogValue, "ok": "fine"}, m["nested"].([]any)[0])
	assert.True(t, strings.HasSuffix(m["long"].(string), truncatedLogSuffix))
}

func TestSanitizeFreeTextLogValue(t *testing.T) {
	got := sanitizeFreeTextLogValue(`call failed Bearer abc.def for bob@example.com password=hunter2`)

	assert.NotContains(t, got, "abc.def")
	assert.NotContains(t, got, "bob@example.com")
	assert.NotContains(t, got, "hunter2")
	assert.Contains(t, got, "call failed")
}

func TestSanitizedBody(t *testing.T) {
	body, truncated := sanitizedBody([]byte(`{"password":"p","a":1}`))
	assert.False(t, truncated)
	assert.Equal(t, map[string]any{"password": redactedLogValue, "a": float64(1)}, body)

	body, _ = sanitizedBody([]byte(`{not json`))
	assert.Equal(t, invalidJSONRedactedLogValue, body)

	body, truncated = sanitizedBody([]byte(`"` + strings.Repeat("a", maxLoggedBodyBytes) + `"`))
	assert.True(t, truncated)
	assert.Equal(t, invalidJSONRedactedLogValue, body)

	body, _ = sanitizedBody(nil)
	assert.Nil(t, body)
}

func TestSanitizeHeadersKeepsOnlyAllowlistAndRedactsSensitive(t *testing.T) {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	h.Set("Authorization", "Bearer x")
	h.Set("X-Other", "ignored")

	got := sanitizeHeaders(h)

	assert.Equal(t, map[string]any{"Content-Type": "application/json", "Authorization": redactedLogValue}, got)
}

func TestSanitizeQuery(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?a=1&a=2&token=z&b=x", nil)

	got := sanitizeQuery(req)

	assert.Equal(t, []any{"1", "2"}, got["a"])
	assert.Equal(t, redactedLogValue, got["token"])
	assert.Equal(t, "x", got["b"])
}
