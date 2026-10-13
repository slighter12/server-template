package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newIPRequest(t *testing.T, headers map[string]string) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:8080"
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return req
}

func TestClientIPExtractor_DirectIgnoresForwardedHeaders(t *testing.T) {
	extractor := NewClientIPExtractor(ClientIPOptions{})
	req := newIPRequest(t, map[string]string{"X-Forwarded-For": "198.51.100.10", cloudflareClientIPHeader: "198.51.100.11"})

	assert.Equal(t, "192.0.2.10", extractor(req))
}

func TestClientIPExtractor_CloudflareRequiresOriginSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		cfIP   string
		want   string
	}{
		{name: "missing secret", cfIP: "198.51.100.10", want: "192.0.2.10"},
		{name: "wrong secret", secret: "wrong", cfIP: "198.51.100.10", want: "192.0.2.10"},
		{name: "matching secret", secret: "origin-secret", cfIP: "198.51.100.10", want: "198.51.100.10"},
		{name: "invalid header", secret: "origin-secret", cfIP: "not-an-ip", want: "192.0.2.10"},
	}
	extractor := NewClientIPExtractor(ClientIPOptions{CloudflareSecret: "origin-secret"})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{cloudflareClientIPHeader: tt.cfIP}
			if tt.secret != "" {
				headers[cloudflareSecretHeader] = tt.secret
			}

			assert.Equal(t, tt.want, extractor(newIPRequest(t, headers)))
		})
	}
}
