# server-template

English | [繁體中文](README.zh-TW.md)

A minimal Go microservice template: **HTTP router (Echo) -> gRPC service (loopback)**, wired with fx in `cmd/server-template/main.go`. Delete one side to get a pure HTTP router or a pure gRPC service (see [AGENTS.md](AGENTS.md)). The auth example (register / login / logout / profile) shows the full path: handler -> HTTP usecase -> gRPC client -> gRPC server -> usecase -> repository (Postgres). See [Features](#features) for what is included.

## Features

- Go 1.27; Echo v5 with h2c; optional HTTP/3.
- gRPC server with health service; reflection is available outside `production`.
- `apperr` unified error handling and a single response envelope ([docs/api-conventions.md](docs/api-conventions.md)).
- OpenTelemetry: generated tracing proxies, `echo-otel`, `otelgrpc`, logs carrying `trace_id` / `span_id`.
- argon2id password hashing.
- Mandatory connections: Postgres, MySQL, MongoDB, Redis (delete the ones you do not use). Optional modules: HTTP/3, debug server (pprof, `goroutineleak`, FlightRecorder).
- CI: lint, race tests, generated-proxy drift check, govulncheck.

## Quick start

Requires Go 1.27 and Docker.

```sh
docker compose up -d --wait   # postgres, mysql, mongo, redis (single-node cluster)
go tool goose -dir database/migrations/postgres postgres \
  "postgres://app:app@localhost:5432/app?sslmode=disable" up
cp config/config_demo.yaml config/config.yaml
go run ./cmd/server-template
```

HTTP listens on `:8080`, gRPC on `:9090`. Emails are normalized to lower-case and unique case-insensitively; passwords must be 8-72 bytes with at least one upper-case letter, one lower-case letter and one digit.

```sh
curl -s localhost:8080/health

curl -s -X POST localhost:8080/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"a@example.com","password":"Passw0rdXyz"}'

TOKEN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"a@example.com","password":"Passw0rdXyz"}' | jq -r .data.token)

curl -s localhost:8080/api/profile -H "Authorization: Bearer $TOKEN"
```

## Configuration

`config.yaml` is searched in `.`, `./config`, `../config`, `../../config`. Start from `config/config_demo.yaml` (`config.yaml` is git-ignored). Environment variables override keys present in the file (key path upper-cased, joined with `_`):

```sh
HTTP_PORT=9000 OBSERVABILITY_OTEL_ENABLE=true AUTH_JWTSECRET="$(openssl rand -base64 48)" go run ./cmd/server-template
```

### Connections

All four connections are mandatory: `config.Validate` requires each section, and the app fails to start when a section is missing or a service does not answer the startup ping. `docker compose up -d --wait` starts matching services for `config/config_demo.yaml`.

| Connection | Config key | Used by | docker-compose service |
| --- | --- | --- | --- |
| Postgres | `postgres` | user repository, transactions | `postgres` (`:5432`) |
| Redis (cluster client) | `redis` | token store and revocation list | `redis` (`:6379`, single-node cluster) |
| MySQL | `mysql` | nothing yet (connection check only) | `mysql` (`:3306`) |
| MongoDB | `mongo` | nothing yet (connection check only) | `mongo` (`:27017`) |

There is no `enable` flag. To drop a connection you do not use, delete its provider registration in `injectConn` (`cmd/server-template/main.go`), its config section (and its `config.Validate` check), its docker-compose service, and any repository that depends on it.

Other modules (defaults from `config/config.go` and `config/config_demo.yaml`):

| Module | Config key | Default | Notes |
| --- | --- | --- | --- |
| HTTP/3 | `http3.enable`, `http3.port` | `false`, `8443` | |
| Debug server | `debugServer.enable`, `debugServer.addr` | `false`, `127.0.0.1:6060` | pprof, `goroutineleak`, FlightRecorder; keep it on loopback. |
| OpenTelemetry | `observability.otel.enable` | `false` | See Observability below. |
| Rate limit | `http.rateLimit.enable`, `requestsPerSecond`, `burst` | `false`, `50`, `100` (demo values) | |
| CORS | `http.corsAllowedOrigins` | `[]` | |

## Observability

```sh
docker compose --profile observability up -d   # Tempo (OTLP gRPC :4317) + Grafana :3000
OBSERVABILITY_OTEL_ENABLE=true go run ./cmd/server-template
```

Send a few requests, open http://localhost:3000, then Explore -> Tempo and search by service name `server-template`. `make otel-verify` runs the scripted end-to-end check ([scripts/verify-otel.sh](scripts/verify-otel.sh)); `SKIP_COMPOSE=1 make otel-verify` skips docker compose when Postgres, MySQL, MongoDB, Redis and Tempo are already reachable. Other overrides: `PG_HOST` `PG_PORT` `PG_USER` `PG_PASSWORD` `PG_DB`, `MYSQL_HOST` `MYSQL_PORT` `MYSQL_USER` `MYSQL_PASSWORD` `MYSQL_DB`, `MONGO_ADDR`, `REDIS_ADDR`, `DSN`, `TEMPO_URL`, `OTLP_ENDPOINT`, `HTTP_PORT`, `GRPC_PORT`, `KEEP_COMPOSE=1`, `POLL_TIMEOUT`. Details: [docs/observability.md](docs/observability.md).

## Testing & CI

```sh
make test-race         # go test -race
make lint              # golangci-lint
make modernize-check   # pending Go modernize fixes; must be empty
make proxy.gen && git diff --exit-code   # generated proxies up to date
make test-integration  # docker compose up -d --wait, then real Postgres/MySQL/MongoDB/Redis checks (-tags=integration)
```

`.github/workflows/ci.yml` runs on pull requests and pushes to `main`: golangci-lint, actionlint, `go test -race ./...`, the proxy drift check above, govulncheck, an `integration` job (`make test-integration`), then a Docker image build.

## Layout

```
cmd/server-template/   entrypoint and fx wiring
cmd/generator/         OTel proxy generator (config: .generator.yaml)
cmd/gen/               migration and gorm/gen model generation (gem)
config/                config loading and validation, config_demo.yaml
database/migrations/   goose migrations (postgres)
deploy/observability/  Tempo and Grafana config for docker compose
docs/                  conventions and guides
internal/delivery/     http (echo), grpc, debug server
internal/usecase/      business logic (+ generated tracing proxies)
internal/domain/       entities, errors, interfaces
internal/repository/   Postgres repositories, RPC repository, connections
internal/infrastructure/  logging, OTel, rpc clients, password hasher
internal/pkg/          apperr, observability context
internal/test/         end-to-end tests (otel, conn integration)
proto/pb/              auth.proto and generated code
scripts/               verify-otel.sh (used by make otel-verify)
```

## Common commands

`make help` lists everything. Frequent ones: `make test-race`, `make lint`, `make proxy.gen`, `make proto.gen`, `make modernize` (apply) / `make modernize-check`, `make otel-verify`, `make gci-format` (`golangci-lint fmt`), `make db-postgres-up` / `db-postgres-down` (non-interactive; override `POSTGRES_DSN`, default matches docker-compose).

## Docs

- [AGENTS.md](AGENTS.md): architecture rules, error handling, verification, turning the template into a service
- [docs/api-conventions.md](docs/api-conventions.md)
- [docs/observability.md](docs/observability.md)
- [docs/go-modernization.md](docs/go-modernization.md)
