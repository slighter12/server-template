# AGENTS.md - server-template Agent Guide

## Purpose

A minimal Go microservice template demonstrating "HTTP router -> gRPC service (loopback)". One binary wires both sides with fx (`cmd/server-template/main.go`). Deleting one side yields a pure HTTP router or a pure gRPC service. The auth feature (register / login / logout / profile) is the worked example.

## Working Rules

- Keep changes minimal; no speculative abstractions or new dependencies unless required.
- Keep repository artifacts in English. Do not commit unless asked.
- Docs: `docs/api-conventions.md` (response envelope, status tables), `docs/observability.md`, `docs/go-modernization.md`.

## Layering

Dependency direction: delivery -> usecase -> domain <- repository / infrastructure.

- `internal/delivery/{http,grpc,debug}`: transport parsing, validation, response and status mapping.
- `internal/usecase`: business orchestration. `*.gen.go` files are generated proxies; do not edit.
- `internal/domain`: entities, sentinels, and interfaces (`repository`, `usecase`, `service`, `delivery`).
- `internal/repository`, `internal/infrastructure`: persistence, RPC clients, logging, OTel, hashing.
- `internal/pkg`: cross-cutting primitives with no layer dependencies (`apperr`, `observability` request context).

## Error Handling

- Define sentinels with `apperr.New(kind, "CODE", "client-safe message")` next to the feature (see `internal/domain/entity/errors.go`). Shared ones live in `internal/pkg/apperr/common.go`.
- Wrap lower-level failures with `apperr.Wrap(cause, sentinel)`. It keeps the cause and captures the stack once; use `errors.Is` / `errors.AsType`.
- Handlers and usecases only `return err`. Never write error bodies in handlers.
- Only delivery maps `Kind` to a transport status: `response.StatusFor` (HTTP) and `kindToCode` in `internal/delivery/grpc/errors.go` (gRPC). The RPC client interceptor maps gRPC status back to `*apperr.Error`.
- Never leak causes or stacks to clients. Server faults (`Internal`, `Unavailable`) are redacted to generic code/message; 401/403 drop `fields`.
- The HTTP request logger (`middleware.RequestLogger`) owns the final log line per request. Do not log the same error again in lower layers.
- Never log secrets, tokens, passwords, or full bodies.

## OpenTelemetry

- Interfaces in `internal/domain/{repository,usecase}` get tracing proxies (`*.gen.go`) from `cmd/generator`, configured in `.generator.yaml`.
- After changing such an interface (or adding one to `.generator.yaml`), run `make proxy.gen`. CI regenerates and fails on `git diff`.
- Proxies are enabled via `fx.Decorate` in `main.go` only when `observability.otel.enable` is true. See `docs/observability.md`.

## Migrations

- Files in `database/migrations/postgres/` are goose files managed by gem; add new files, never edit existing ones.
- gem currently emits MySQL `ALTER` syntax. For Postgres, write `ALTER` migrations by hand (see `20261013000000_alter_users.sql`).
- Postgres string columns default to `TEXT`; length limits live in request validation (or an explicit `CHECK` when the DB must enforce it).

## Verification

```sh
go build ./...
go test -race ./...            # or: make test-race
golangci-lint run ./... --config=./.golangci.yaml   # or: make lint
make modernize-check           # go fix -diff ./...; must be empty
make proxy.gen && git diff --exit-code   # generated proxies up to date
make otel-verify               # OTel instrumentation check
```

Docs-only changes: verify referenced paths manually.

## Turning the Template into a Service

`main.go` is grouped into `inject*` options. fx tolerates unused providers, but remove what you do not need.

### Pure HTTP router (remote gRPC backend)

1. In `main.go` remove `injectRepo()`, `injectUseCase()`, `injectGRPC()` and the imports they leave unused (`grpc`, `repository`, `usecase`/`repo` as needed). Keep `rpc.New` in `injectConn` and drop DB providers you do not use.
2. Delete `internal/delivery/grpc/`, `internal/usecase/auth.go`, `auth.gen.go`, `auth_test.go`, `internal/domain/usecase/auth.go`, `internal/repository/{user*.go,transaction*.go,gen/,conn/}` (unused connections), `internal/domain/repository/{user.go,transaction.go}`, `internal/infrastructure/auth/`, and the `UserRepository` / `AuthUseCase` entries in `.generator.yaml` (then `make proxy.gen`).
3. Point `rpc.clients.auth.target` at the remote service. Note `config.Validate` still requires `grpc.port`, `auth.*` and `rpc.server.target`; relax it in `config/config.go`.

### Pure gRPC service

1. In `main.go` remove `injectHTTP()`, the HTTP/`rpc` imports, and `rpc.New` from `injectConn`.
2. Delete `internal/delivery/http/`, `internal/usecase/auth_http*.go`, `internal/domain/usecase/auth_http.go`, `internal/repository/auth_rpc.go`, `internal/domain/repository/auth_rpc.go`, `internal/infrastructure/rpc/`, and the `AuthHTTPUseCase` entry in `.generator.yaml` (then `make proxy.gen`).
3. Remove now-unused config (`http*`, `rpc`) and `config.Validate` checks for them; `go mod tidy`.

Keep `injectInfra`, `injectDebug`, `startServer`, and `internal/pkg` in both cases. Finish with the verification commands above.
