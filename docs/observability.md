# Observability

## Tracing design

- Domain interfaces (`UserRepository`, `AuthUseCase`, `AuthHTTPUseCase`) get generated tracing proxies: `cmd/generator` + `cmd/generator/templates/otel_proxy.tmpl`, configured by `.generator.yaml`, output in `*.gen.go`. Regenerate with `make proxy.gen`.
- Span status is apperr-aware: every error span gets `app.error_code` and `app.error_kind`; only server faults (`Internal`, `Unavailable`) set span status Error and add an `exception` event. The error message is never stored (it may contain PII).
- Transport spans: `echo-otel` middleware for HTTP (`server.New`), `otelgrpc` stats handlers for the gRPC server and client, so a request is one trace across the loopback hop. The `traceparent` response header exposes the trace id to clients.
- Logs carry `trace_id` / `span_id` (`otel.NewTraceHandler`, wired only when OTel is enabled). The handler is the single source: the request logger does not add them itself, so keys are never duplicated.
- Span names and attributes follow the stable RPC semconv emitted by `otelgrpc` 0.72: gRPC spans are named after the full method (`auth.v1.Auth/Login`), `rpc.method` holds that full method, `rpc.system.name` is `grpc`, and there is no `rpc.service`. HTTP server spans are `<METHOD> <route>` (for example `POST /auth/login`) with `http.route`. Generated proxy spans are `Interface.Method` (for example `AuthUseCase.Login`) with `code.function`.
- Everything is off by default; with OTel disabled the proxies and middleware are not installed.

## Configuration

Keys in `config/config_demo.yaml`, overridable by env var (key path upper-cased, joined with `_`):

| Key | Env var |
| --- | --- |
| `observability.otel.enable` | `OBSERVABILITY_OTEL_ENABLE` |
| `observability.otel.exporter` (`otlp-grpc` or `otlp-http`) | `OBSERVABILITY_OTEL_EXPORTER` |
| `observability.otel.host` / `port` | `OBSERVABILITY_OTEL_HOST` / `OBSERVABILITY_OTEL_PORT` |
| `observability.otel.isSecure` | `OBSERVABILITY_OTEL_ISSECURE` |
| `env.serviceName` | `ENV_SERVICENAME` |

## Run locally

```sh
docker compose --profile observability up -d --wait   # Tempo (OTLP gRPC :4317) + Grafana :3000, plus the mandatory postgres/mysql/mongo/redis
cp config/config_demo.yaml config/config.yaml
OBSERVABILITY_OTEL_ENABLE=true go run ./cmd/server-template
```

Send a few requests (see README), open http://localhost:3000, then Explore -> Tempo datasource -> search by service name `server-template`, or paste a trace id from a log line or the `traceparent` header. `make otel-verify` runs the scripted check.

## Debug server

Off by default. Enable with `debugServer.enable: true` (`DEBUGSERVER_ENABLE=true`); it listens on `debugServer.addr` (default `127.0.0.1:6060`; keep it on loopback).

- `/debug/pprof/` and the standard profiles, including the Go 1.27 `goroutineleak` profile: `/debug/pprof/goroutineleak`.
- `GET /debug/flightrecorder`: downloads the last ~10s (up to 16 MiB) of execution trace from `runtime/trace.FlightRecorder`; open with `go tool trace`.

Pyroscope continuous profiling is separate (`observability.pyroscope.*`).
