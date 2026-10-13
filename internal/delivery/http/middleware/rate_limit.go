package middleware

import (
	"errors"
	"fmt"
	"time"

	"server-template/internal/pkg/apperr"

	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
)

// RateLimitOptions configures NewRateLimiter. A non-positive Rate disables it.
type RateLimitOptions struct {
	// Rate is the sustained number of requests per second per key.
	Rate float64
	// Burst is the maximum number of requests allowed at once per key.
	Burst int
	// ExpiresIn is how long an idle key is remembered.
	ExpiresIn time.Duration
}

// Enabled reports whether the limiter should be installed.
func (o RateLimitOptions) Enabled() bool {
	return o.Rate > 0
}

// Validate checks an enabled configuration.
func (o RateLimitOptions) Validate() error {
	if !o.Enabled() {
		return nil
	}
	if o.Burst < 1 {
		return fmt.Errorf("rate limit burst must be at least 1, got %d", o.Burst)
	}
	if o.ExpiresIn <= 0 {
		return errors.New("rate limit expiresIn must be positive")
	}

	return nil
}

// KeyByIP limits per client IP.
func KeyByIP(c *echo.Context) string {
	return c.RealIP()
}

// NewRateLimiter returns an in-process token-bucket limiter keyed by keyFn
// (KeyByIP when nil). Denied requests return apperr.ErrTooManyRequests. It
// returns a nil middleware when the options are disabled.
func NewRateLimiter(opts RateLimitOptions, keyFn func(*echo.Context) string) (echo.MiddlewareFunc, error) {
	if !opts.Enabled() {
		return nil, nil
	}
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid rate limit configuration: %w", err)
	}
	if keyFn == nil {
		keyFn = KeyByIP
	}

	store := echomiddleware.NewRateLimiterMemoryStoreWithConfig(echomiddleware.RateLimiterMemoryStoreConfig{
		Rate:      opts.Rate,
		Burst:     opts.Burst,
		ExpiresIn: opts.ExpiresIn,
	})

	return echomiddleware.RateLimiterConfig{
		Store: store,
		IdentifierExtractor: func(c *echo.Context) (string, error) {
			return keyFn(c), nil
		},
		DenyHandler: func(_ *echo.Context, _ string, err error) error {
			return apperr.Wrap(err, apperr.ErrTooManyRequests)
		},
	}.ToMiddleware()
}
