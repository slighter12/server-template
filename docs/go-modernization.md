# Go Modernization

Go features this template uses (verified in source). `make modernize-check` (`go fix -diff ./...`) should stay empty.

| Go | Feature | Where |
| --- | --- | --- |
| 1.27 | stdlib `uuid` (`uuid.NewV7`) | `internal/delivery/http/middleware/request_id.go`, `internal/delivery/http/request/request.go` |
| 1.27 | struct literal keys for promoted fields (applied by `go fix`) | `internal/delivery/grpc/jwt.go` (`Claims{ExpiresAt: ...}`) |
| 1.27 | `http.Server.MaxHeaderValueCount` | `internal/delivery/http/http2/server.go` (config `http.maxHeaderValueCount`) |
| 1.27 | `goroutineleak` pprof profile | `internal/delivery/debug/server.go`, tested in `server_test.go` |
| 1.27 | `httptest.NewTestServer` with `synctest` | `internal/delivery/http/http2/server_test.go` |
| 1.26 | `errors.AsType` | `internal/pkg/apperr/apperr.go`, `internal/delivery/http/middleware/errors.go`, `internal/delivery/grpc/errors.go` |
| 1.26 | `new(expr)` | `internal/usecase/auth_http.go` |
| 1.25 | `sync.WaitGroup.Go` | `cmd/server-template/main.go` |
| 1.25 | `runtime/trace.FlightRecorder` | `internal/delivery/debug/server.go` |
| 1.25 | `testing/synctest` (`synctest.Test`) | `http2/server_test.go`, `internal/infrastructure/auth/argon2id_hasher_test.go` |
| 1.24 | `http.Protocols` (h2c without x/net) | `internal/delivery/http/http2/server.go` |
| 1.24 | `go.mod` `tool` directive (goose, govulncheck, protoc plugins) | `go.mod`, `Makefile` |
| 1.24 | `b.Loop` | `internal/infrastructure/auth/argon2id_hasher_test.go` |
| 1.24 | `t.Context()` | `internal/delivery/debug/server_test.go`, `http3/server_test.go`, `server/server_test.go` |
| 1.24 | `omitzero` JSON tag, `encoding/json/v2` | `internal/delivery/http/response/` |
| 1.23 | `slices.Sorted(maps.Keys(...))` | `internal/infrastructure/rpc/client.go`, `cmd/generator/main.go` |
| 1.22 | range over int | `internal/delivery/http/middleware/request_id.go`, `cmd/generator/main.go` |

## Follow-ups

- Back-port the `apperr` design (Kind, sentinels, redaction, single status tables) to NomNom-Radar.
- NomNom-Radar remaining modernization spots:
  - `errors.As` -> `errors.AsType`: `internal/infra/persistence/postgres/constraint_errors.go:24-25`
  - `sort.Strings` -> `slices.Sorted(maps.Keys(...))`: `cmd/routing/regions.go:43-47`
  - index loop -> range: `internal/domain/policy/policy.go:50`
  - batching loops -> `slices.Chunk`: `internal/delivery/worker/handler/push_handler.go:366`, `internal/usecase/impl/notification_service.go:281`
  - `time.Sleep` in server tests -> `synctest`
  - `context.WithCancel` in `argon2id_hasher_test.go:97` -> `t.Context()`
- gem emits MySQL `ALTER` syntax even for Postgres; ALTER migrations are hand-converted until upstream is fixed.
