.PHONY: help test-race test-integration lint sec-scan gci-format \
	db-postgres-init db-postgres-seeders-init \
	build docker-image-build \
	db-postgres-down db-postgres-up \
	proto.gen proxy.gen modernize modernize-check otel-verify

help: ## show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

PROJECT_NAME ?=
SQL_FILE_TIMESTAMP := $(shell date '+%Y%m%d%H%M%S')
GitCommit := $(shell git rev-parse HEAD)
Date := $(shell date -Iseconds)
SHELL := /bin/bash
GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0

########
# test #
########

test-race: ## launch all tests with race detection
	go test -p 4 ./... -cover -race

test-integration: ## start postgres/mysql/mongo/redis (docker compose) and run the integration tests (they apply the goose migrations themselves)
	docker compose up -d --wait postgres mysql mongo redis
	go test -tags=integration -race -count=1 ./internal/test/conn/... ./internal/delivery/grpc/...

########
# lint #
########

lint: ## lints the entire codebase
	@$(GOLANGCI_LINT) run ./... --config=./.golangci.yaml

#######
# sec #
#######

sec-scan: trivy-scan vuln-scan ## scan for security and vulnerability issues

trivy-scan: ## scan for sec issues with trivy (trivy binary needed)
	trivy fs --exit-code 1 --no-progress --severity CRITICAL ./

vuln-scan: ## scan for vulnerability issues with govulncheck (go tool)
	go tool govulncheck ./...

###########
# modern  #
###########

modernize: ## apply Go modernize fixes across the codebase
	go fix ./...

modernize-check: ## show pending Go modernize fixes without applying them
	go fix -diff ./...

##########
# verify #
##########

otel-verify: ## verify OpenTelemetry instrumentation
	./scripts/verify-otel.sh

######
# db #
######
POSTGRES_SQL_PATH := ./database/migrations/postgres
POSTGRES_SEEDERS_SQL_PATH := ./database/migrations/postgres/seeders
# Defaults match docker-compose.yml; override with `make db-postgres-up POSTGRES_DSN=...`.
POSTGRES_DSN ?= postgres://app:app@localhost:5432/app?sslmode=disable

# -----------------------------------------------------------------------------
# PostgreSQL
# -----------------------------------------------------------------------------

db-postgres-init: ## initialize new PostgreSQL migration
	@mkdir -p ${POSTGRES_SQL_PATH}
	go tool goose create init sql -dir ${POSTGRES_SQL_PATH}

db-postgres-seeders-init: ## initialize new PostgreSQL seeder
	@mkdir -p ${POSTGRES_SEEDERS_SQL_PATH}
	@( \
		printf "Enter seeder name: "; read -r SEEDER_NAME && \
		touch ${POSTGRES_SEEDERS_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$${SEEDER_NAME}.up.sql && \
		touch ${POSTGRES_SEEDERS_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$${SEEDER_NAME}.down.sql \
	)

db-postgres-create: ## create new PostgreSQL migration
	@mkdir -p ${POSTGRES_SQL_PATH}
	@( \
		printf "Enter migration name: "; read -r MIGRATE_NAME && \
		go tool goose create $${MIGRATE_NAME} sql -dir ${POSTGRES_SQL_PATH} \
	)

db-postgres-up: ## apply all PostgreSQL migrations (override POSTGRES_DSN)
	go tool goose -dir ${POSTGRES_SQL_PATH} postgres "${POSTGRES_DSN}" up

db-postgres-down: ## revert the last PostgreSQL migration (override POSTGRES_DSN)
	go tool goose -dir ${POSTGRES_SQL_PATH} postgres "${POSTGRES_DSN}" down

##########
# format #
##########

gci-format: ## format code and imports (golangci-lint fmt, uses .golangci.yaml)
	$(GOLANGCI_LINT) fmt --config=./.golangci.yaml ./...

#########
# build #
#########

build: ## build the project
	@( \
		printf "Enter version: "; read -r VERSION; \
		go build -ldflags "-s -w -X 'main.Version=$$VERSION'" -o ./bin/$(PROJECT_NAME) ./cmd/$(PROJECT_NAME) \
	)

docker-image-build: ## build Docker image
	docker build \
		-t $(PROJECT_NAME) \
		--platform $(DOCKER_PLATFORM) \
		--build-arg BUILT=$(Date) \
		--build-arg GIT_COMMIT=$(GitCommit) \
		./

DOCKER_PLATFORM ?= linux/amd64

proto.gen: ## generate protobuf code
	protoc --proto_path=proto/pb \
		--plugin=protoc-gen-go="$$(go tool -n protoc-gen-go)" \
		--plugin=protoc-gen-go-grpc="$$(go tool -n protoc-gen-go-grpc)" \
		--go_out=proto/pb/authpb \
		--go_opt=paths=source_relative \
		--go-grpc_out=proto/pb/authpb \
		--go-grpc_opt=paths=source_relative \
		--go_opt=default_api_level=API_OPAQUE \
		--experimental_allow_proto3_optional \
		proto/pb/auth.proto

proxy.gen: ## generate proxy code
	go run ./cmd/generator --config=.generator.yaml
