# server-template

A minimal Go microservice template: **HTTP router (Echo) -> gRPC service (loopback)**, wired with fx in `cmd/server-template/main.go`. Delete one side to get a pure HTTP router or a pure gRPC service (see [AGENTS.md](AGENTS.md)). The auth example (register / login / logout / profile) shows the full path: handler -> HTTP usecase -> gRPC client -> gRPC server -> usecase -> repository (Postgres).

## Quick start

Requires Go 1.27 and Docker.

```sh
docker compose up -d postgres
go tool goose -dir database/migrations/postgres postgres \
  "postgres://app:app@localhost:5432/app?sslmode=disable" up
cp config/config_demo.yaml config/config.yaml
go run ./cmd/server-template
```

HTTP listens on `:8080`, gRPC on `:9090`.

```sh
curl -s localhost:8080/health

curl -s -X POST localhost:8080/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"a@example.com","password":"Passw0rdXyz"}'

TOKEN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"a@example.com","password":"Passw0rdXyz"}' | jq -r .data.token)

curl -s localhost:8080/api/profile -H "Authorization: Bearer $TOKEN"
```

Redis is optional (`docker compose --profile redis up -d`); without it, token storage and revocation are skipped.

## Configuration

`config.yaml` is searched in `.`, `./config`, `../config`, `../../config`. Start from `config/config_demo.yaml` (`config.yaml` is git-ignored). Environment variables override keys present in the file (key path upper-cased, joined with `_`):

```sh
HTTP_PORT=9000 OBSERVABILITY_OTEL_ENABLE=true AUTH_JWTSECRET="$(openssl rand -base64 48)" go run ./cmd/server-template
```

## Layout

```
cmd/server-template/   entrypoint and fx wiring
cmd/generator/         OTel proxy generator (config: .generator.yaml)
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
proto/pb/              auth.proto and generated code
```

## Common commands

`make help` lists everything. Frequent ones: `make test-race`, `make lint`, `make proxy.gen`, `make proto.gen`, `make modernize-check`.

## Docs

- [AGENTS.md](AGENTS.md): architecture rules, error handling, verification, turning the template into a service
- [docs/api-conventions.md](docs/api-conventions.md)
- [docs/observability.md](docs/observability.md)
- [docs/go-modernization.md](docs/go-modernization.md)
