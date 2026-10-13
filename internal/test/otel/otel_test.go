// Package otel_test is an in-process integration test of the tracing wiring:
// real Echo server + middleware, real router, real proxies, real gRPC server,
// real rpc client conn (interceptors and otelgrpc handlers), real argon2id
// hasher, with only the UserRepository faked. Spans land in an in-memory
// exporter, so the test needs no network beyond a loopback gRPC listener and
// no database.
package otel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"server-template/config"
	"server-template/internal/delivery/grpc"
	"server-template/internal/delivery/http/router"
	"server-template/internal/delivery/http/server"
	"server-template/internal/domain/delivery"
	"server-template/internal/domain/entity"
	repo "server-template/internal/domain/repository"
	use "server-template/internal/domain/usecase"
	"server-template/internal/infrastructure/auth"
	otelinfra "server-template/internal/infrastructure/observability/otel"
	"server-template/internal/infrastructure/rpc"
	"server-template/internal/repository"
	"server-template/internal/usecase"

	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

const (
	testEmail    = "alice@example.com"
	testPassword = "Passw0rdTest"
	// secretCause stands in for a driver error that leaks credentials/PII; it
	// must never reach a span.
	secretCause = "dial tcp 10.1.2.3:5432: password authentication failed for user hunter2-secret"
)

// fakeUserRepo is an in-memory repo.UserRepository. failFind makes FindByEmail
// return a raw (non-apperr) error.
type fakeUserRepo struct {
	mu       sync.Mutex
	byEmail  map[string]*entity.User
	byID     map[string]*entity.User
	failFind error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byEmail: map[string]*entity.User{}, byID: map[string]*entity.User{}}
}

func (f *fakeUserRepo) Create(_ context.Context, u *entity.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.byEmail[u.Email]; ok {
		return entity.ErrEmailTaken
	}
	cp := *u
	cp.CreatedAt = time.Now()
	f.byEmail[u.Email], f.byID[u.ID] = &cp, &cp

	return nil
}

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*entity.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.failFind != nil {
		return nil, f.failFind
	}
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}

	return nil, entity.ErrUserNotFound
}

func (f *fakeUserRepo) FindByID(_ context.Context, id string) (*entity.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if u, ok := f.byID[id]; ok {
		return u, nil
	}

	return nil, entity.ErrUserNotFound
}

func (f *fakeUserRepo) setFailFind(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failFind = err
}

// lockedBuffer is a goroutine-safe log sink.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) lines() []map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()

	var out []map[string]any
	for line := range strings.SplitSeq(b.buf.String(), "\n") {
		if line == "" {
			continue
		}
		m := map[string]any{}
		if err := json.Unmarshal([]byte(line), &m); err == nil {
			out = append(out, m)
		}
	}

	return out
}

// harness is a fully wired app.
type harness struct {
	t        *testing.T
	echo     *echo.Echo
	exporter *tracetest.InMemoryExporter // nil when tracing is disabled
	repo     *fakeUserRepo
	logs     *lockedBuffer
	authUC   use.AuthHTTPUseCase
	userRepo repo.UserRepository
	authSrv  use.AuthUseCase
}

// freePort returns a currently free loopback TCP port.
func freePort(t *testing.T) int {
	t.Helper()

	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	port, ok := l.Addr().(*net.TCPAddr)
	require.True(t, ok)

	return port.Port
}

// newHarness wires the same constructors and decorators as cmd/server-template/main.go.
func newHarness(t *testing.T, otelEnabled bool) *harness {
	t.Helper()

	h := &harness{t: t, repo: newFakeUserRepo(), logs: &lockedBuffer{}}

	if otelEnabled {
		h.exporter = tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(h.exporter)),
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
		)
		prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
		t.Cleanup(func() {
			_ = tp.Shutdown(context.Background())
			otel.SetTracerProvider(prevTP)
			otel.SetTextMapPropagator(prevProp)
		})
	}

	grpcPort := freePort(t)
	cfg := &config.Config{}
	cfg.Env.Env = "test"
	cfg.Env.ServiceName = "server-template-test"
	cfg.Env.Debug = true // request_logger also logs successful requests
	cfg.HTTP.Port = 8080
	cfg.HTTP.MaxRequestBodySize = "1M"
	cfg.GRPC.Port = grpcPort
	cfg.Auth.JWTSecret = strings.Repeat("s", 40)
	cfg.Auth.AccessTokenTTL = time.Hour
	cfg.Auth.Argon2 = config.Argon2Config{Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 16, MaxConcurrent: 2}
	cfg.Observability.Otel.Enable = otelEnabled
	cfg.RPC.Server.Target = fmt.Sprintf("127.0.0.1:%d", grpcPort)
	cfg.RPC.Clients = map[string]config.RPCClientConfig{"auth": {Target: cfg.RPC.Server.Target}}

	var handler slog.Handler = slog.NewJSONHandler(h.logs, &slog.HandlerOptions{Level: slog.LevelDebug})
	if otelEnabled {
		handler = otelinfra.NewTraceHandler(handler)
	}
	logger := slog.New(handler)

	app := fxtest.New(t,
		fx.NopLogger,
		fx.Supply(cfg, logger),
		fx.Provide(
			auth.NewArgon2idHasher,
			rpc.New,
			func() repo.UserRepository { return h.repo },
			func() *redis.ClusterClient { return nil }, // Redis not configured
			usecase.NewAuthUseCase,
			repository.NewAuthRPC,
			usecase.NewAuthHTTPUseCase,
			server.New,
			fx.Annotate(
				router.New,
				fx.As(new(server.RouteRegistrar)),
				fx.ResultTags(`group:"http_routes"`),
			),
			fx.Annotate(grpc.NewGRPC, fx.ResultTags(`group:"deliveries"`)),
		),
		fx.Decorate(
			func(base repo.UserRepository) repo.UserRepository {
				return repository.ProvideUserRepositoryProxy(cfg.Observability.Otel.Enable, base)
			},
			func(base use.AuthUseCase) use.AuthUseCase {
				return usecase.ProvideAuthUseCaseProxy(cfg.Observability.Otel.Enable, base)
			},
			func(base use.AuthHTTPUseCase) use.AuthHTTPUseCase {
				return usecase.ProvideAuthHTTPUseCaseProxy(cfg.Observability.Otel.Enable, base)
			},
		),
		fx.Invoke(func(lc fx.Lifecycle, e *echo.Echo, uc use.AuthHTTPUseCase, ur repo.UserRepository, a use.AuthUseCase,
			deliveries struct {
				fx.In
				D []delivery.Delivery `group:"deliveries"`
			},
		) {
			h.echo, h.authUC, h.userRepo, h.authSrv = e, uc, ur, a

			ctx, cancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			lc.Append(fx.Hook{
				OnStart: func(context.Context) error {
					for _, d := range deliveries.D {
						wg.Go(func() { _ = d.Serve(ctx) })
					}

					return nil
				},
				OnStop: func(context.Context) error {
					cancel()

					return nil
				},
			})
		}),
	)
	app.RequireStart()
	t.Cleanup(app.RequireStop)

	// Wait for the gRPC listener.
	require.Eventually(t, func() bool {
		d := net.Dialer{Timeout: 100 * time.Millisecond}
		c, err := d.DialContext(t.Context(), "tcp", cfg.RPC.Server.Target)
		if err != nil {
			return false
		}
		_ = c.Close()

		return true
	}, 5*time.Second, 10*time.Millisecond)

	return h
}

type result struct {
	*httptest.ResponseRecorder
}

func (h *harness) do(method, target, body string, headers ...string) result {
	h.t.Helper()

	req := httptest.NewRequestWithContext(h.t.Context(), method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.echo.ServeHTTP(rec, req)

	return result{rec}
}

func (r result) json() map[string]any {
	m := map[string]any{}
	_ = json.Unmarshal(r.Body.Bytes(), &m)

	return m
}

// token digs data.token out of the response envelope.
func (r result) token(t *testing.T) string {
	t.Helper()

	data, ok := r.json()["data"].(map[string]any)
	require.True(t, ok, "no data in %s", r.Body.String())
	tok, ok := data["token"].(string)
	require.True(t, ok, "no token in %s", r.Body.String())

	return tok
}

func (h *harness) register(email, password string) result {
	h.t.Helper()

	return h.do(http.MethodPost, "/auth/register", fmt.Sprintf(`{"email":%q,"password":%q}`, email, password))
}

func (h *harness) login(email, password string) result {
	h.t.Helper()

	return h.do(http.MethodPost, "/auth/login", fmt.Sprintf(`{"email":%q,"password":%q}`, email, password))
}

// reset drops everything recorded so far (e.g. seeding traffic).
func (h *harness) reset() { h.exporter.Reset() }

// spans waits until at least n spans were exported, then (after a short settle
// period, to catch stragglers) returns them. gRPC server spans end
// asynchronously relative to the client's response.
func (h *harness) spans(n int) []tracetest.SpanStub {
	h.t.Helper()

	require.Eventually(h.t, func() bool { return len(h.exporter.GetSpans()) >= n }, 5*time.Second, 5*time.Millisecond,
		"expected at least %d spans, got %d", n, len(h.exporter.GetSpans()))
	time.Sleep(50 * time.Millisecond)

	return h.exporter.GetSpans()
}

func byName(t *testing.T, spans []tracetest.SpanStub, name string) tracetest.SpanStub {
	t.Helper()

	var found []tracetest.SpanStub
	for _, s := range spans {
		if s.Name == name {
			found = append(found, s)
		}
	}
	require.Len(t, found, 1, "want exactly one span named %q; have %v", name, spanNames(spans))

	return found[0]
}

func spanNames(spans []tracetest.SpanStub) []string {
	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.Name)
	}

	return names
}

func attrOf(s tracetest.SpanStub, key string) (attribute.Value, bool) {
	for _, kv := range s.Attributes {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}

	return attribute.Value{}, false
}

func attrString(t *testing.T, s tracetest.SpanStub, key string) string {
	t.Helper()

	v, ok := attrOf(s, key)
	require.True(t, ok, "span %q has no attribute %q; has %v", s.Name, key, s.Attributes)

	return v.Emit()
}

// httpServerSpan returns the single server span created by echo-otel.
func httpServerSpan(t *testing.T, spans []tracetest.SpanStub, method, route string) tracetest.SpanStub {
	t.Helper()

	for _, s := range spans {
		if s.SpanKind == oteltrace.SpanKindServer && !s.Parent.IsRemote() &&
			strings.HasPrefix(s.Name, method) && strings.Contains(s.Name, route) {
			return s
		}
	}
	require.FailNow(t, "HTTP server span not found", "spans: %v", spanNames(spans))

	return tracetest.SpanStub{}
}

func requireChild(t *testing.T, child, parent tracetest.SpanStub) {
	t.Helper()

	assert.Equal(t, parent.SpanContext.SpanID(), child.Parent.SpanID(),
		"%q should be a child of %q", child.Name, parent.Name)
}

func requireSingleTrace(t *testing.T, spans []tracetest.SpanStub) oteltrace.TraceID {
	t.Helper()

	require.NotEmpty(t, spans)
	tid := spans[0].SpanContext.TraceID()
	for _, s := range spans {
		assert.Equal(t, tid, s.SpanContext.TraceID(), "span %q left the trace; all spans: %v", s.Name, spanNames(spans))
	}

	return tid
}

func grpcServerSpan(t *testing.T, spans []tracetest.SpanStub, fullMethod string) tracetest.SpanStub {
	t.Helper()

	for _, s := range spans {
		if s.Name == fullMethod && s.SpanKind == oteltrace.SpanKindServer {
			return s
		}
	}
	require.FailNow(t, "gRPC server span not found", "want %q in %v", fullMethod, spanNames(spans))

	return tracetest.SpanStub{}
}

func grpcClientSpan(t *testing.T, spans []tracetest.SpanStub, fullMethod string) tracetest.SpanStub {
	t.Helper()

	for _, s := range spans {
		if s.Name == fullMethod && s.SpanKind == oteltrace.SpanKindClient {
			return s
		}
	}
	require.FailNow(t, "gRPC client span not found", "want %q in %v", fullMethod, spanNames(spans))

	return tracetest.SpanStub{}
}

// TestLoginTraceChain covers spec 7.1 A1, A2, A3 and the traceparent header.
func TestLoginTraceChain(t *testing.T) {
	h := newHarness(t, true)
	require.Equal(t, http.StatusCreated, h.register(testEmail, testPassword).Code)
	h.reset()

	rec := h.login(testEmail, testPassword)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	spans := h.spans(6)

	// 1. one trace, exactly the six expected spans.
	tid := requireSingleTrace(t, spans)
	require.Len(t, spans, 6, "unexpected spans: %v", spanNames(spans))

	httpSpan := httpServerSpan(t, spans, http.MethodPost, "/auth/login")
	httpUC := byName(t, spans, "AuthHTTPUseCase.Login")
	rpcClient := grpcClientSpan(t, spans, "auth.v1.Auth/Login")
	rpcServer := grpcServerSpan(t, spans, "auth.v1.Auth/Login")
	authUC := byName(t, spans, "AuthUseCase.Login")
	repoSpan := byName(t, spans, "UserRepository.FindByEmail")

	// 2. parent/child chain across the loopback gRPC boundary.
	assert.False(t, httpSpan.Parent.IsValid(), "HTTP server span must be the root")
	requireChild(t, httpUC, httpSpan)
	requireChild(t, rpcClient, httpUC)
	requireChild(t, rpcServer, rpcClient)
	requireChild(t, authUC, rpcServer)
	requireChild(t, repoSpan, authUC)
	assert.True(t, rpcServer.Parent.IsRemote(), "server span's parent arrived over the wire")

	// 3. attributes.
	assert.Equal(t, "/auth/login", attrString(t, httpSpan, "http.route"))
	assert.Equal(t, "Login", attrString(t, httpUC, "code.function"))
	assert.Equal(t, "Login", attrString(t, authUC, "code.function"))
	assert.Equal(t, "FindByEmail", attrString(t, repoSpan, "code.function"))
	for _, s := range []tracetest.SpanStub{rpcClient, rpcServer} {
		// otelgrpc >= 0.6x uses the stable RPC semconv: rpc.method is the full
		// "package.Service/Method" and rpc.service is no longer emitted.
		assert.Equal(t, "auth.v1.Auth/Login", attrString(t, s, "rpc.method"), s.Name)
		assert.Equal(t, "grpc", attrString(t, s, "rpc.system.name"), s.Name)
	}

	// Happy path: nothing is in error.
	for _, s := range spans {
		assert.NotEqual(t, codes.Error, s.Status.Code, "span %q", s.Name)
	}

	// traceparent response header matches the trace.
	tp := rec.Header().Get("Traceparent")
	require.NotEmpty(t, tp)
	parts := strings.Split(tp, "-")
	require.Len(t, parts, 4, tp)
	assert.Equal(t, tid.String(), parts[1])
	assert.Equal(t, httpSpan.SpanContext.SpanID().String(), parts[2])
}

// TestInvalidCredentialsIsNotSpanError covers A4 (client error).
func TestInvalidCredentialsIsNotSpanError(t *testing.T) {
	h := newHarness(t, true)
	require.Equal(t, http.StatusCreated, h.register(testEmail, testPassword).Code)
	h.reset()

	rec := h.login(testEmail, "Wr0ngPasswordX")
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())

	spans := h.spans(6)
	requireSingleTrace(t, spans)

	for _, name := range []string{"AuthHTTPUseCase.Login", "AuthUseCase.Login"} {
		s := byName(t, spans, name)
		assert.NotEqual(t, codes.Error, s.Status.Code, "client error must not mark %q as Error", name)
		assert.Equal(t, "INVALID_CREDENTIALS", attrString(t, s, "app.error_code"), name)
	}
	// The repository call itself succeeded.
	assert.NotEqual(t, codes.Error, byName(t, spans, "UserRepository.FindByEmail").Status.Code)
	// gRPC spans carry a status code of Unauthenticated, which otelgrpc does not treat as a span error on the server.
	assert.NotEqual(t, codes.Error, grpcServerSpan(t, spans, "auth.v1.Auth/Login").Status.Code)
}

// TestInternalErrorMarksSpanAndHidesCause covers A4 (server fault) and PII.
func TestInternalErrorMarksSpanAndHidesCause(t *testing.T) {
	h := newHarness(t, true)
	require.Equal(t, http.StatusCreated, h.register(testEmail, testPassword).Code)
	h.reset()
	h.repo.setFailFind(errors.New(secretCause))

	rec := h.login(testEmail, testPassword)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "hunter2", "secret leaked to the HTTP client")

	spans := h.spans(6)
	requireSingleTrace(t, spans)

	for _, name := range []string{"UserRepository.FindByEmail", "AuthUseCase.Login", "AuthHTTPUseCase.Login"} {
		s := byName(t, spans, name)
		assert.Equal(t, codes.Error, s.Status.Code, name)
		assert.Equal(t, "INTERNAL_ERROR", s.Status.Description, name)
		assert.Equal(t, "INTERNAL_ERROR", attrString(t, s, "app.error_code"), name)
	}

	// The raw error text appears in no span: attributes, events, status, links, name.
	for _, s := range spans {
		dump := fmt.Sprintf("%s|%s|%+v|%+v|%+v", s.Name, s.Status.Description, s.Attributes, s.Events, s.Links)
		assert.NotContains(t, dump, "hunter2", "span %q", s.Name)
		assert.NotContains(t, dump, "10.1.2.3", "span %q", s.Name)
		assert.NotContains(t, dump, "password authentication", "span %q", s.Name)
	}
}

// TestProfileSingleTraceWithValidateToken covers A5.
func TestProfileSingleTraceWithValidateToken(t *testing.T) {
	h := newHarness(t, true)
	token := h.register(testEmail, testPassword).token(t)
	h.reset()

	rec := h.do(http.MethodGet, "/api/profile", "", "Authorization", "Bearer "+token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), testEmail)

	// HTTP server, AuthHTTPUseCase.ValidateToken, client, server, AuthUseCase.GetUserByID, UserRepository.FindByID.
	spans := h.spans(6)
	tid := requireSingleTrace(t, spans)
	require.Len(t, spans, 6, "unexpected spans: %v", spanNames(spans))

	httpSpan := httpServerSpan(t, spans, http.MethodGet, "/api/profile")
	httpUC := byName(t, spans, "AuthHTTPUseCase.ValidateToken")
	rpcClient := grpcClientSpan(t, spans, "auth.v1.Auth/ValidateToken")
	rpcServer := grpcServerSpan(t, spans, "auth.v1.Auth/ValidateToken")
	getUser := byName(t, spans, "AuthUseCase.GetUserByID")
	findByID := byName(t, spans, "UserRepository.FindByID")

	assert.Equal(t, "/api/profile", attrString(t, httpSpan, "http.route"))
	requireChild(t, httpUC, httpSpan)
	requireChild(t, rpcClient, httpUC)
	requireChild(t, rpcServer, rpcClient)
	requireChild(t, getUser, rpcServer)
	requireChild(t, findByID, getUser)

	parts := strings.Split(rec.Header().Get("Traceparent"), "-")
	require.Len(t, parts, 4)
	assert.Equal(t, tid.String(), parts[1])
}

// TestRequestLogCarriesTraceID covers A6 and the trace-handler for in-flight logs.
func TestRequestLogCarriesTraceID(t *testing.T) {
	h := newHarness(t, true)
	require.Equal(t, http.StatusCreated, h.register(testEmail, testPassword).Code)
	h.reset()

	rec := h.login(testEmail, "Wr0ngPasswordX")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	spans := h.spans(6)
	httpSpan := httpServerSpan(t, spans, http.MethodPost, "/auth/login")

	var entry map[string]any
	for _, l := range h.logs.lines() {
		if l["msg"] == "HTTP request" && l["path"] == "/auth/login" && l["status"] == float64(http.StatusUnauthorized) {
			entry = l
		}
	}
	require.NotNil(t, entry, "request_logger line not found in %v", h.logs.lines())
	assert.Equal(t, httpSpan.SpanContext.TraceID().String(), entry["trace_id"])
	assert.Equal(t, httpSpan.SpanContext.SpanID().String(), entry["span_id"])
	assert.NotEmpty(t, entry["request_id"])
	assert.NotContains(t, fmt.Sprint(entry), testPassword, "password leaked into the log line")
}

// TestOtelDisabled covers A7.
func TestOtelDisabled(t *testing.T) {
	// Install a recording provider globally anyway: with enable=false nothing
	// may use it.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})

	h := newHarness(t, false)

	// Proxies are not applied: the decorated values are the bare implementations.
	assert.NotContains(t, fmt.Sprintf("%T", h.authUC), "Proxy")
	assert.NotContains(t, fmt.Sprintf("%T", h.authSrv), "Proxy")
	assert.NotContains(t, fmt.Sprintf("%T", h.userRepo), "Proxy")

	token := h.register(testEmail, testPassword).token(t)
	rec := h.login(testEmail, testPassword)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, rec.Header().Get("Traceparent"))
	assert.Equal(t, http.StatusOK, h.do(http.MethodGet, "/api/profile", "", "Authorization", "Bearer "+token).Code)
	assert.Equal(t, http.StatusUnauthorized, h.login(testEmail, "Wr0ngPasswordX").Code)

	time.Sleep(100 * time.Millisecond)
	assert.Empty(t, exporter.GetSpans(), "tracing disabled must not produce spans")
}
