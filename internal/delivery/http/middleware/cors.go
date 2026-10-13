package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
)

// CORSOptions configures NewCORS.
type CORSOptions struct {
	// AllowedOrigins is a comma-separated list of exact http(s) origins.
	// Empty disables CORS entirely (fail closed).
	AllowedOrigins string
}

// NewCORS builds a fail-closed CORS middleware from exact origins. It returns
// a nil middleware when no origin is configured.
func NewCORS(opts CORSOptions) (echo.MiddlewareFunc, error) {
	origins, err := parseCORSAllowedOrigins(opts.AllowedOrigins)
	if err != nil {
		return nil, err
	}
	if len(origins) == 0 {
		return nil, nil
	}

	return echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: origins,
		AllowMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPut,
			http.MethodPatch,
			http.MethodPost,
			http.MethodDelete,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
		},
		AllowCredentials: false,
	}), nil
}

func parseCORSAllowedOrigins(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var origins []string
	seen := make(map[string]struct{})
	for part := range strings.SplitSeq(raw, ",") {
		origin := strings.TrimSpace(part)
		if origin == "" {
			return nil, errors.New("CORS origin list contains an empty origin")
		}

		canonical, err := canonicalizeCORSOrigin(origin)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		origins = append(origins, canonical)
	}

	return origins, nil
}

func canonicalizeCORSOrigin(origin string) (string, error) {
	if strings.ContainsAny(origin, "*?") {
		return "", fmt.Errorf("CORS origin %q must not contain wildcard characters", origin)
	}

	parsed, err := url.Parse(origin)
	if err != nil || !isValidCORSOriginURL(parsed) {
		return "", fmt.Errorf("CORS origin %q must be an http or https origin", origin)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("CORS origin %q must use http or https", origin)
	}
	if parsed.Hostname() == "" {
		return "", fmt.Errorf("CORS origin %q must include a host", origin)
	}

	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func isValidCORSOriginURL(parsed *url.URL) bool {
	return parsed.Scheme != "" &&
		parsed.Host != "" &&
		parsed.User == nil &&
		parsed.Path == "" &&
		parsed.RawPath == "" &&
		parsed.RawQuery == "" &&
		parsed.Fragment == ""
}
