# server-template

[English](README.md) | 繁體中文

一個精簡的 Go 微服務範本：**HTTP router (Echo) -> gRPC service（loopback）**，由 `cmd/server-template/main.go` 以 fx 組裝。刪除其中一側即可得到純 HTTP router 或純 gRPC service（見 [AGENTS.md](AGENTS.md)）。auth 範例（register / login / logout / profile）展示完整路徑：handler -> HTTP usecase -> gRPC client -> gRPC server -> usecase -> repository（Postgres）。功能總覽見 [Features](#features)。

## Features

- Go 1.27；Echo v5 搭配 h2c；可選 HTTP/3。
- gRPC server 含 health service；非 `production` 環境可開啟 reflection。
- `apperr` 統一錯誤處理與單一回應信封（[docs/api-conventions.md](docs/api-conventions.md)）。
- OpenTelemetry：產生的 tracing proxy、`echo-otel`、`otelgrpc`、log 帶 `trace_id` / `span_id`。
- argon2id 密碼雜湊。
- 選配模組：HTTP/3、Redis、MySQL、MongoDB、debug server（pprof、`goroutineleak`、FlightRecorder）。
- CI：lint、race 測試、proxy 產生檔漂移檢查、govulncheck。

## Quick start

需要 Go 1.27 與 Docker。

```sh
docker compose up -d postgres
go tool goose -dir database/migrations/postgres postgres \
  "postgres://app:app@localhost:5432/app?sslmode=disable" up
cp config/config_demo.yaml config/config.yaml
go run ./cmd/server-template
```

HTTP 監聽 `:8080`，gRPC 監聽 `:9090`。Email 會正規化為小寫，且唯一性不分大小寫；密碼須為 8-72 bytes，且至少包含一個大寫字母、一個小寫字母與一個數字。

```sh
curl -s localhost:8080/health

curl -s -X POST localhost:8080/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"a@example.com","password":"Passw0rdXyz"}'

TOKEN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"a@example.com","password":"Passw0rdXyz"}' | jq -r .data.token)

curl -s localhost:8080/api/profile -H "Authorization: Bearer $TOKEN"
```

## Configuration

`config.yaml` 會在 `.`、`./config`、`../config`、`../../config` 中搜尋。請從 `config/config_demo.yaml` 開始（`config.yaml` 已被 git 忽略）。環境變數可覆蓋檔案中存在的 key（key 路徑轉大寫，以 `_` 連接）：

```sh
HTTP_PORT=9000 OBSERVABILITY_OTEL_ENABLE=true AUTH_JWTSECRET="$(openssl rand -base64 48)" go run ./cmd/server-template
```

選配模組（預設值取自 `config/config.go` 與 `config/config_demo.yaml`）：

| 模組 | Config key | 預設值 | 說明 |
| --- | --- | --- | --- |
| HTTP/3 | `http3.enable`、`http3.port` | `false`、`8443` | |
| Redis | `redis` 區段 | 不存在（關閉） | 在設定檔中取消註解；`docker compose --profile redis up -d` 可啟動一個。未啟用時，token 儲存與撤銷會被略過。 |
| MySQL | `mysql` 區段 | 不存在（關閉） | 在設定檔中取消註解；移除區段即停用。 |
| MongoDB | `mongo` 區段 | 不存在（關閉） | 在設定檔中取消註解。 |
| Debug server | `debugServer.enable`、`debugServer.addr` | `false`、`127.0.0.1:6060` | pprof、`goroutineleak`、FlightRecorder；請維持在 loopback。 |
| OpenTelemetry | `observability.otel.enable` | `false` | 見下方 Observability。 |
| Rate limit | `http.rateLimit.enable`、`requestsPerSecond`、`burst` | `false`、`50`、`100`（demo 值） | |
| CORS | `http.corsAllowedOrigins` | `[]` | |

## Observability

```sh
docker compose --profile observability up -d   # Tempo (OTLP gRPC :4317) + Grafana :3000
OBSERVABILITY_OTEL_ENABLE=true go run ./cmd/server-template
```

送出幾個請求後開啟 http://localhost:3000，進入 Explore -> Tempo，以 service name `server-template` 搜尋。`make otel-verify` 會執行腳本化的端到端檢查（[scripts/verify-otel.sh](scripts/verify-otel.sh)）；若 Postgres 與 Tempo 已可連線，`SKIP_COMPOSE=1 make otel-verify` 會略過 docker compose。其他覆蓋變數：`PG_HOST` `PG_PORT` `PG_USER` `PG_PASSWORD` `PG_DB`、`DSN`、`TEMPO_URL`、`OTLP_ENDPOINT`、`HTTP_PORT`、`GRPC_PORT`、`KEEP_COMPOSE=1`、`POLL_TIMEOUT`。詳見 [docs/observability.md](docs/observability.md)。

## Testing & CI

```sh
make test-race         # go test -race
make lint              # golangci-lint
make modernize-check   # 列出待套用的 Go modernize 修正；必須為空
make proxy.gen && git diff --exit-code   # 產生的 proxy 為最新
```

`.github/workflows/ci.yml` 在 pull request 與推送到 `main` 時執行：golangci-lint、actionlint、`go test -race ./...`、上述 proxy 漂移檢查、govulncheck，最後建置 Docker image。

## Layout

```
cmd/server-template/   進入點與 fx 組裝
cmd/generator/         OTel proxy 產生器（設定：.generator.yaml）
cmd/gen/               migration 與 gorm/gen model 產生（gem）
config/                設定載入與驗證、config_demo.yaml
database/migrations/   goose migrations（postgres）
deploy/observability/  docker compose 用的 Tempo 與 Grafana 設定
docs/                  慣例與指南
internal/delivery/     http (echo)、grpc、debug server
internal/usecase/      業務邏輯（含產生的 tracing proxy）
internal/domain/       entity、錯誤、介面
internal/repository/   Postgres repository、RPC repository、連線
internal/infrastructure/  logging、OTel、rpc client、密碼雜湊
internal/pkg/          apperr、observability context
internal/test/         端到端測試（otel）
proto/pb/              auth.proto 與產生的程式碼
scripts/               verify-otel.sh（供 make otel-verify 使用）
```

## Common commands

`make help` 會列出所有指令。常用：`make test-race`、`make lint`、`make proxy.gen`、`make proto.gen`、`make modernize`（套用）/ `make modernize-check`、`make otel-verify`、`make gci-format`（`golangci-lint fmt`）、`make db-postgres-up` / `db-postgres-down`（非互動式；可覆蓋 `POSTGRES_DSN`，預設值與 docker-compose 一致）。

## Docs

- [AGENTS.md](AGENTS.md)：架構規則、錯誤處理、驗證、將範本改成服務
- [docs/api-conventions.md](docs/api-conventions.md)
- [docs/observability.md](docs/observability.md)
- [docs/go-modernization.md](docs/go-modernization.md)
