// Package server builds the single Echo instance shared by the HTTP/1.1+h2c
// and HTTP/3 listeners, so both transports run the same middleware and routes.
package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"server-template/config"
	"server-template/internal/delivery/http/middleware"
	"server-template/internal/delivery/http/response"
	"server-template/internal/delivery/http/validator"

	echootel "github.com/labstack/echo-otel/v5"
	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
	"go.uber.org/fx"
)

// RoutesGroup is the fx value group routers provide themselves into.
const RoutesGroup = "http_routes"

// rateLimitIdleKeyTTL is how long an idle client key stays in the limiter.
const rateLimitIdleKeyTTL = 3 * time.Minute

// RouteRegistrar registers routes on the shared Echo instance. Provide an
// implementation with fx.Annotate(..., fx.As(new(server.RouteRegistrar)),
// fx.ResultTags(`group:"http_routes"`)); removing the router leaves a server
// that only serves /health.
type RouteRegistrar interface {
	Register(e *echo.Echo)
}

// Params are the dependencies of New.
type Params struct {
	fx.In

	Config *config.Config
	Logger *slog.Logger
	Routes []RouteRegistrar `group:"http_routes"`
}

// Module provides the shared *echo.Echo.
var Module = fx.Module("http-server", fx.Provide(New))

// New builds the shared Echo instance.
//
// Middleware order: Recover, RequestID, echo-otel (OTel enabled only),
// Traceparent (OTel enabled only), RequestLogger, HandleErrors, CORS (when
// origins are configured), rate limit (when enabled), BodyLimit,
// CaptureRequestBody. AltSvc is prepended when HTTP/3 is enabled. It returns an
// error for invalid CORS, rate-limit or body-limit configuration.
func New(p Params) (*echo.Echo, error) {
	cfg := p.Config

	bodyLimit, err := parseSize(cfg.HTTP.MaxRequestBodySize)
	if err != nil {
		return nil, fmt.Errorf("http.maxRequestBodySize: %w", err)
	}
	cors, err := middleware.NewCORS(middleware.CORSOptions{AllowedOrigins: strings.Join(cfg.HTTP.CORSAllowedOrigins, ",")})
	if err != nil {
		return nil, fmt.Errorf("configure CORS: %w", err)
	}
	limiter, err := middleware.NewRateLimiter(middleware.RateLimitOptions{
		Rate:      cfg.HTTP.RateLimit.RequestsPerSecond,
		Burst:     cfg.HTTP.RateLimit.Burst,
		ExpiresIn: rateLimitIdleKeyTTL,
	}, middleware.KeyByIP)
	if err != nil {
		return nil, fmt.Errorf("configure rate limiter: %w", err)
	}
	if !cfg.HTTP.RateLimit.Enable {
		limiter = nil
	}

	e := echo.New()
	e.Logger = p.Logger
	e.JSONSerializer = response.NewJSONSerializer()
	e.HTTPErrorHandler = middleware.HTTPErrorHandler
	e.Validator = validator.New()
	e.IPExtractor = middleware.NewClientIPExtractor(middleware.ClientIPOptions{CloudflareSecret: cfg.HTTP.CloudflareSecret})

	chain := []echo.MiddlewareFunc{
		echomiddleware.Recover(),
		middleware.RequestID(p.Logger),
	}
	if cfg.Observability.Otel.Enable {
		chain = append(chain, echootel.NewMiddleware(cfg.Env.ServiceName), middleware.Traceparent())
	}
	chain = append(chain,
		middleware.RequestLogger(p.Logger, middleware.RequestLoggerOptions{Debug: cfg.Env.Debug}),
		middleware.HandleErrors(),
	)
	if cfg.HTTP3.Enable {
		chain = append(chain, middleware.AltSvc(cfg.HTTP3.Port))
	}
	if cors != nil {
		chain = append(chain, cors)
	}
	if limiter != nil {
		chain = append(chain, limiter)
	}
	chain = append(chain, echomiddleware.BodyLimit(bodyLimit), middleware.CaptureRequestBody())
	e.Use(chain...)

	e.GET("/health", health)
	for _, r := range p.Routes {
		r.Register(e)
	}

	return e, nil
}

// health is the liveness probe.
func health(c *echo.Context) error {
	return response.Success(c, http.StatusOK, map[string]string{"status": "ok"})
}

// parseSize parses echo body-limit notation: a number with an optional
// K, M, G, T or P suffix (case-insensitive, optional trailing "B"; 1024-based).
func parseSize(s string) (int64, error) {
	raw := strings.ToUpper(strings.TrimSpace(s))
	raw = strings.TrimSuffix(raw, "B")
	if raw == "" {
		return 0, errors.New("empty size")
	}

	multiplier := int64(1)
	if i := strings.IndexByte("KMGTP", raw[len(raw)-1]); i >= 0 {
		multiplier = int64(1) << (10 * (i + 1))
		raw = raw[:len(raw)-1]
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid size %q", s)
	}
	if n > (1<<62)/multiplier {
		return 0, fmt.Errorf("size %q is too large", s)
	}

	return n * multiplier, nil
}
