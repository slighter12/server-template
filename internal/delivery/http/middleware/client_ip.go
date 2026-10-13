package middleware

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

const (
	cloudflareClientIPHeader = "CF-Connecting-IP"
	// #nosec G101 -- This is a public HTTP header name, not a credential.
	cloudflareSecretHeader = "X-Cloudflare-Secret"
)

// ClientIPOptions configures NewClientIPExtractor.
type ClientIPOptions struct {
	// CloudflareSecret, when set, makes requests that present it in the
	// X-Cloudflare-Secret header trust CF-Connecting-IP. Empty means direct.
	CloudflareSecret string
}

// NewClientIPExtractor returns the extractor for e.IPExtractor. Cloudflare
// origin traffic uses its authenticated client-IP header; everything else
// ignores forwarded headers and uses the network peer address.
func NewClientIPExtractor(opts ClientIPOptions) echo.IPExtractor {
	direct := echo.ExtractIPDirect()
	secret := strings.TrimSpace(opts.CloudflareSecret)
	if secret == "" {
		return direct
	}

	return func(req *http.Request) string {
		if !isAuthenticatedCloudflareRequest(req, secret) {
			return direct(req)
		}
		if ip := net.ParseIP(strings.TrimSpace(req.Header.Get(cloudflareClientIPHeader))); ip != nil {
			return ip.String()
		}

		return direct(req)
	}
}

func isAuthenticatedCloudflareRequest(req *http.Request, expectedSecret string) bool {
	if req == nil || strings.TrimSpace(expectedSecret) == "" {
		return false
	}
	got := strings.TrimSpace(req.Header.Get(cloudflareSecretHeader))

	return len(got) == len(expectedSecret) && subtle.ConstantTimeCompare([]byte(got), []byte(expectedSecret)) == 1
}
