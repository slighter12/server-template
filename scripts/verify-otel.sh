#!/usr/bin/env bash
# End-to-end OpenTelemetry check against a real Tempo.
#
# Flow: start postgres, mysql, mongo, redis + tempo (docker compose, profile
# "observability"; all four connections are mandatory for the app) ->
# goose migrations -> run the app with OTel enabled -> register / login /
# GET /api/profile -> read the trace id from the `traceparent` response header
# -> poll Tempo until the trace has the expected spans and service.name.
#
# Usage:  scripts/verify-otel.sh        (or: make otel-verify)
#
# Environment overrides (all optional):
#   SKIP_COMPOSE=1        do not run docker compose; Postgres, MySQL, Mongo,
#                         Redis and Tempo must already be reachable (used where
#                         there is no docker).
#   PG_HOST PG_PORT PG_USER PG_PASSWORD PG_DB
#                         Postgres connection (defaults: localhost 5432 app app app).
#   MYSQL_HOST MYSQL_PORT MYSQL_USER MYSQL_PASSWORD MYSQL_DB
#                         MySQL connection (defaults: localhost 3306 app app app).
#   MONGO_ADDR            Mongo host:port (default localhost:27017).
#   REDIS_ADDR            Redis cluster node host:port (default localhost:6379).
#   DSN                   goose connection string (default derived from PG_*).
#   TEMPO_URL             Tempo query API (default http://localhost:3200).
#   OTLP_ENDPOINT         host:port of Tempo's OTLP gRPC receiver (default localhost:4317).
#   HTTP_PORT GRPC_PORT   ports the app listens on (defaults 8080 / 9090).
#   KEEP_COMPOSE=1        leave the compose services running on exit.
#   POLL_TIMEOUT          seconds to wait for the trace in Tempo (default 30).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SKIP_COMPOSE="${SKIP_COMPOSE:-0}"
KEEP_COMPOSE="${KEEP_COMPOSE:-0}"
PG_HOST="${PG_HOST:-localhost}"
PG_PORT="${PG_PORT:-5432}"
PG_USER="${PG_USER:-app}"
PG_PASSWORD="${PG_PASSWORD:-app}"
PG_DB="${PG_DB:-app}"
MYSQL_HOST="${MYSQL_HOST:-localhost}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-app}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-app}"
MYSQL_DB="${MYSQL_DB:-app}"
MONGO_ADDR="${MONGO_ADDR:-localhost:27017}"
REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
DSN="${DSN:-postgres://${PG_USER}:${PG_PASSWORD}@${PG_HOST}:${PG_PORT}/${PG_DB}?sslmode=disable}"
TEMPO_URL="${TEMPO_URL:-http://localhost:3200}"
OTLP_ENDPOINT="${OTLP_ENDPOINT:-localhost:4317}"
OTLP_HOST="${OTLP_ENDPOINT%:*}"
OTLP_PORT="${OTLP_ENDPOINT##*:}"
HTTP_PORT="${HTTP_PORT:-8080}"
GRPC_PORT="${GRPC_PORT:-9090}"
POLL_TIMEOUT="${POLL_TIMEOUT:-30}"
SERVICE_NAME="server-template"
BASE_URL="http://127.0.0.1:${HTTP_PORT}"

WORK_DIR="$(mktemp -d)"
APP_LOG="${WORK_DIR}/app.log"
APP_PID=""
COMPOSE=(docker compose --profile observability)

log() { printf '\033[36m[verify-otel]\033[0m %s\n' "$*"; }
fail() { printf '\033[31m[verify-otel] FAIL:\033[0m %s\n' "$*" >&2; exit 1; }

dump_app_log() {
  if [[ -s "$APP_LOG" ]]; then
    echo "----- app log (${APP_LOG}) -----" >&2
    tail -n 80 "$APP_LOG" >&2 || true
  fi
}

cleanup() {
  local status=$?
  set +e
  if [[ -n "$APP_PID" ]] && kill -0 "$APP_PID" 2>/dev/null; then
    kill "$APP_PID" 2>/dev/null
    for _ in $(seq 1 50); do kill -0 "$APP_PID" 2>/dev/null || break; sleep 0.1; done
    kill -9 "$APP_PID" 2>/dev/null
  fi
  if [[ "$SKIP_COMPOSE" != "1" && "$KEEP_COMPOSE" != "1" ]]; then
    "${COMPOSE[@]}" down >/dev/null 2>&1
  fi
  if [[ $status -eq 0 ]]; then rm -rf "$WORK_DIR"; else echo "work dir kept: $WORK_DIR" >&2; fi
  exit $status
}
trap cleanup EXIT
trap 'exit 130' INT TERM

for tool in curl jq go; do
  command -v "$tool" >/dev/null || fail "$tool is required"
done
[[ "$SKIP_COMPOSE" == "1" ]] || command -v docker >/dev/null || fail "docker is required (or set SKIP_COMPOSE=1)"

# wait_for <description> <timeout-seconds> <command...>
wait_for() {
  local what=$1 timeout=$2
  shift 2
  local deadline=$((SECONDS + timeout))
  until "$@" >/dev/null 2>&1; do
    ((SECONDS < deadline)) || fail "timed out after ${timeout}s waiting for ${what}"
    sleep 1
  done
}

tcp_open() { (exec 3<>"/dev/tcp/$1/$2") 2>/dev/null; }

# 1. infrastructure ---------------------------------------------------------
if [[ "$SKIP_COMPOSE" == "1" ]]; then
  log "SKIP_COMPOSE=1: using existing services (Postgres ${PG_HOST}:${PG_PORT}, MySQL ${MYSQL_HOST}:${MYSQL_PORT}, Mongo ${MONGO_ADDR}, Redis ${REDIS_ADDR}, Tempo ${TEMPO_URL})"
else
  log "starting postgres, mysql, mongo, redis and tempo"
  "${COMPOSE[@]}" up -d --wait postgres mysql mongo redis tempo
fi
wait_for "postgres on ${PG_HOST}:${PG_PORT}" 60 tcp_open "$PG_HOST" "$PG_PORT"
wait_for "mysql on ${MYSQL_HOST}:${MYSQL_PORT}" 60 tcp_open "$MYSQL_HOST" "$MYSQL_PORT"
wait_for "mongo on ${MONGO_ADDR}" 60 tcp_open "${MONGO_ADDR%:*}" "${MONGO_ADDR##*:}"
wait_for "redis on ${REDIS_ADDR}" 60 tcp_open "${REDIS_ADDR%:*}" "${REDIS_ADDR##*:}"
wait_for "tempo ready (${TEMPO_URL}/ready)" 90 curl -fsS "${TEMPO_URL}/ready"
wait_for "tempo OTLP receiver on ${OTLP_ENDPOINT}" 30 tcp_open "$OTLP_HOST" "$OTLP_PORT"

# 2. migrations -------------------------------------------------------------
log "running goose migrations"
go tool goose -dir database/migrations/postgres postgres "$DSN" up

# 3. app --------------------------------------------------------------------
# Build once and run from a scratch dir holding config.yaml (the app reads
# ./config.yaml), so the working tree stays clean.
log "building app"
go build -o "${WORK_DIR}/server-template" ./cmd/server-template
sed -E \
  -e "s/^(  port: )8080$/\1${HTTP_PORT}/" \
  -e "s/^(  port: )9090$/\1${GRPC_PORT}/" \
  -e "s#localhost:9090#localhost:${GRPC_PORT}#g" \
  config/config_demo.yaml |
  awk \
    -v pg_host="$PG_HOST" -v pg_port="$PG_PORT" -v pg_user="$PG_USER" -v pg_pass="$PG_PASSWORD" -v pg_db="$PG_DB" \
    -v my_host="$MYSQL_HOST" -v my_port="$MYSQL_PORT" -v my_user="$MYSQL_USER" -v my_pass="$MYSQL_PASSWORD" -v my_db="$MYSQL_DB" \
    -v mongo_addr="$MONGO_ADDR" -v redis_addr="$REDIS_ADDR" '
    /^[a-z]/ { section = $1; sub(/:.*/, "", section) }
    section == "postgres" && /^    database: / { print "    database: \"" pg_db "\""; next }
    section == "postgres" && /^      host: /     { print "      host: \"" pg_host "\""; next }
    section == "postgres" && /^      port: /     { print "      port: \"" pg_port "\""; next }
    section == "postgres" && /^      username: / { print "      username: \"" pg_user "\""; next }
    section == "postgres" && /^      password: / { print "      password: \"" pg_pass "\""; next }
    section == "mysql" && /^    database: /     { print "    database: \"" my_db "\""; next }
    section == "mysql" && /^      host: /       { print "      host: \"" my_host "\""; next }
    section == "mysql" && /^      port: /       { print "      port: \"" my_port "\""; next }
    section == "mysql" && /^      username: /   { print "      username: \"" my_user "\""; next }
    section == "mysql" && /^      password: /   { print "      password: \"" my_pass "\""; next }
    section == "mongo" && /^      - "/          { print "      - \"" mongo_addr "\""; next }
    section == "redis" && /^    - "/            { print "    - \"" redis_addr "\""; next }
    { print }
  ' >"${WORK_DIR}/config.yaml"

log "starting app (OTel -> ${OTLP_ENDPOINT}, http :${HTTP_PORT}, grpc :${GRPC_PORT})"
(
  cd "$WORK_DIR"
  # Viper applies env overrides to keys present in config.yaml (see config.LoadWithEnv).
  export OBSERVABILITY_OTEL_ENABLE=true \
    OBSERVABILITY_OTEL_EXPORTER=otlp-grpc \
    OBSERVABILITY_OTEL_HOST="$OTLP_HOST" \
    OBSERVABILITY_OTEL_PORT="$OTLP_PORT" \
    OBSERVABILITY_OTEL_ISSECURE=false \
    ENV_DEBUG=true
  exec ./server-template
) >"$APP_LOG" 2>&1 &
APP_PID=$!

app_ready() { kill -0 "$APP_PID" 2>/dev/null || { dump_app_log; fail "app exited during startup"; }; curl -fsS "${BASE_URL}/health"; }
wait_for "app /health" 60 app_ready

# 4. traffic ----------------------------------------------------------------
EMAIL="otel-$(date +%s)-$RANDOM@example.com"
PASSWORD="Passw0rdOtel1"

# call <name> <method> <path> [curl args...]: prints body to $WORK_DIR/<name>.body,
# headers to $WORK_DIR/<name>.hdr, echoes the HTTP status.
call() {
  local name=$1 method=$2 path=$3
  shift 3
  curl -sS -o "${WORK_DIR}/${name}.body" -D "${WORK_DIR}/${name}.hdr" -w '%{http_code}' \
    -X "$method" -H 'Content-Type: application/json' "$@" "${BASE_URL}${path}"
}

trace_id_of() { # <name>
  local tp
  tp=$(grep -i '^traceparent:' "${WORK_DIR}/$1.hdr" | tr -d '\r' | awk '{print $2}') || true
  [[ -n "$tp" ]] || fail "no traceparent header in $1 response"
  echo "$tp" | cut -d- -f2
}

expect_status() { # <name> <got> <want>
  [[ "$2" == "$3" ]] || { cat "${WORK_DIR}/$1.body" >&2; dump_app_log; fail "$1: HTTP $2, want $3"; }
}

log "register -> login -> profile as ${EMAIL}"
code=$(call register POST /auth/register -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")
expect_status register "$code" 201
code=$(call login POST /auth/login -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}")
expect_status login "$code" 200
TOKEN=$(jq -er '.data.token' "${WORK_DIR}/login.body") || fail "no token in login response"
code=$(call profile GET /api/profile -H "Authorization: Bearer ${TOKEN}")
expect_status profile "$code" 200

LOGIN_TRACE=$(trace_id_of login)
PROFILE_TRACE=$(trace_id_of profile)
log "login trace id:   ${LOGIN_TRACE}"
log "profile trace id: ${PROFILE_TRACE}"

# 5. assert against Tempo ---------------------------------------------------
# Tempo's v1 JSON is {"batches":[{"resource":..,"scopeSpans":[{"spans":[..]}]}]};
# newer builds may wrap it as {"trace":{"resourceSpans":[..]}}.
SPAN_NAMES_JQ='[(.batches // .trace.resourceSpans // [])[] | (.scopeSpans // .instrumentationLibrarySpans // [])[] | .spans[]?.name] | unique'
SERVICES_JQ='[(.batches // .trace.resourceSpans // [])[] | .resource.attributes[]? | select(.key=="service.name") | .value.stringValue] | unique'
SPAN_COUNT_JQ='[(.batches // .trace.resourceSpans // [])[] | (.scopeSpans // .instrumentationLibrarySpans // [])[] | .spans[]?] | length'

# check_trace <label> <trace-id> <min-spans> <expected-span-name>...
check_trace() {
  local label=$1 id=$2 min=$3
  shift 3
  local expected
  expected=$(printf '%s\n' "$@" | jq -R . | jq -s .)
  local deadline=$((SECONDS + POLL_TIMEOUT)) body="${WORK_DIR}/tempo-${label}.json" http=000

  log "polling Tempo for ${label} trace (up to ${POLL_TIMEOUT}s)"
  while :; do
    http=$(curl -sS -o "$body" -w '%{http_code}' -H 'Accept: application/json' "${TEMPO_URL}/api/traces/${id}" || echo 000)
    if [[ "$http" == "200" ]] && jq -e \
      --argjson want "$expected" --argjson min "$min" --arg svc "$SERVICE_NAME" \
      "(${SPAN_NAMES_JQ}) as \$names | (${SERVICES_JQ}) as \$svcs | (${SPAN_COUNT_JQ}) as \$n
       | (\$want - \$names | length == 0) and (\$svcs | index(\$svc) != null) and (\$n >= \$min)" \
      "$body" >/dev/null 2>&1; then
      log "${label}: OK  service.name=$(jq -c "$SERVICES_JQ" "$body")  spans=$(jq "$SPAN_COUNT_JQ" "$body")"
      log "${label}: span names $(jq -c "$SPAN_NAMES_JQ" "$body")"
      return 0
    fi
    if ((SECONDS >= deadline)); then
      echo "----- Tempo response for ${label} (HTTP ${http}) -----" >&2
      cat "$body" >&2 || true
      echo >&2
      echo "expected span names: ${expected}  min spans: ${min}  service.name: ${SERVICE_NAME}" >&2
      dump_app_log
      fail "${label} trace ${id} not complete in Tempo after ${POLL_TIMEOUT}s"
    fi
    sleep 1
  done
}

check_trace login "$LOGIN_TRACE" 6 \
  "POST /auth/login" "AuthHTTPUseCase.Login" "auth.v1.Auth/Login" \
  "AuthUseCase.Login" "UserRepository.FindByEmail"
check_trace profile "$PROFILE_TRACE" 6 \
  "GET /api/profile" "AuthHTTPUseCase.ValidateToken" "auth.v1.Auth/ValidateToken" \
  "AuthUseCase.GetUserByID" "UserRepository.FindByID"

log "SUCCESS: traces reached Tempo with the expected spans"
