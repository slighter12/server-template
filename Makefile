.PHONY: help test-race lint sec-scan gci-format \
	db-mysql-init db-mysql-seeders-init db-postgres-init db-postgres-seeders-init \
	build docker-image-build \
	db-mysql-down db-mysql-up db-postgres-down db-postgres-up gen-migrate-sql \
	proto.gen proxy.gen

help: ## show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

PROJECT_NAME ?=
SQL_FILE_TIMESTAMP := $(shell date '+%Y%m%d%H%M%S')
GitCommit := $(shell git rev-parse HEAD)
Date := $(shell date -Iseconds)
SHELL := /bin/bash

########
# test #
########

test-race: ## launch all tests with race detection
	go test -p 4 ./... -cover -race

########
# lint #
########

lint: ## lints the entire codebase
	@golangci-lint run ./... --config=./.golangci.yaml

#######
# sec #
#######

sec-scan: trivy-scan vuln-scan ## scan for security and vulnerability issues

trivy-scan: ## scan for sec issues with trivy (trivy binary needed)
	trivy fs --exit-code 1 --no-progress --severity CRITICAL ./

vuln-scan: ## scan for vulnerability issues with govulncheck (govulncheck binary needed)
	govulncheck ./...

######
# db #
######
MYSQL_SQL_PATH := ./database/migrations/mysql
MYSQL_SEEDERS_SQL_PATH := ./database/migrations/mysql/seeders
POSTGRES_SQL_PATH := ./database/migrations/postgres
POSTGRES_SEEDERS_SQL_PATH := ./database/migrations/postgres/seeders

# -----------------------------------------------------------------------------
# MySQL
# -----------------------------------------------------------------------------

db-mysql-init: ## initialize new MySQL migration
	@mkdir -p ${MYSQL_SQL_PATH}
	@( \
		printf "Enter migrate name: "; read -r MIGRATE_NAME && \
		migrate create -ext sql -dir ${MYSQL_SQL_PATH} $${MIGRATE_NAME} \
	)

db-mysql-seeders-init: ## initialize new MySQL seeder
	@mkdir -p ${MYSQL_SEEDERS_SQL_PATH}
	@( \
		printf "Enter seeder name: "; read -r SEEDER_NAME && \
		touch ${MYSQL_SEEDERS_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$${SEEDER_NAME}.up.sql && \
		touch ${MYSQL_SEEDERS_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$${SEEDER_NAME}.down.sql \
	)

define migrate_command
	migrate -database "mysql://root:$(1)@tcp(127.0.0.1:$(2))/$(3)?multiStatements=true" -path $(4) $(5)
endef

db-mysql-up: ## apply all MySQL migrations, including seeders
	@( \
		printf "Enter database name: "; read -r DB_NAME && \
		printf "Enter pass for db: "; read -rs DB_PASSWORD && \
		echo && \
		printf "Enter port(3306...): "; read -r DB_PORT && \
		: "$${DB_PORT:=3306}" && \
		$(call migrate_command,$$DB_PASSWORD,$$DB_PORT,$$DB_NAME,${MYSQL_SQL_PATH},up) && \
		$(call migrate_command,$$DB_PASSWORD,$$DB_PORT,$$DB_NAME,${MYSQL_SEEDERS_SQL_PATH},up) \
	)

db-mysql-down: ## revert all MySQL migrations, including seeders
	@( \
		printf "Enter database name: "; read -r DB_NAME && \
		printf "Enter pass for db: "; read -rs DB_PASSWORD && \
		echo && \
		printf "Enter port(3306...): "; read -r DB_PORT && \
		: "$${DB_PORT:=3306}" && \
		$(call migrate_command,$$DB_PASSWORD,$$DB_PORT,$$DB_NAME,${MYSQL_SEEDERS_SQL_PATH},down) && \
		$(call migrate_command,$$DB_PASSWORD,$$DB_PORT,$$DB_NAME,${MYSQL_SQL_PATH},down) \
	)

# -----------------------------------------------------------------------------
# PostgreSQL
# -----------------------------------------------------------------------------

db-postgres-init: ## initialize new PostgreSQL migration
	@mkdir -p ${POSTGRES_SQL_PATH}
	goose create init sql -dir ${POSTGRES_SQL_PATH}

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
		goose create $${MIGRATE_NAME} sql -dir ${POSTGRES_SQL_PATH} \
	)

define goose_migrate_command
	PG_URI="postgres://postgres:$(PG_PASSWORD)@localhost:$(PG_PORT)/$(DB_NAME)?sslmode=disable"
	goose postgres -dir $(1) $(2) $${PG_URI}
endef

db-postgres-up: ## apply all PostgreSQL migrations
	@( \
		printf "Enter database name: "; read -r DB_NAME && \
		printf "Enter pass for db: "; read -rs PG_PASSWORD && \
		printf "Enter port(5432...): "; read -r PG_PORT && \
		PG_PORT=$${PG_PORT:-5432} && \
		PG_URI="postgres://root:$${PG_PASSWORD}@localhost:$${PG_PORT}/$${DB_NAME}?sslmode=disable" && \
		goose postgres "$${PG_URI}" -dir ${POSTGRES_SQL_PATH} up \
	)

db-postgres-down: ## revert all PostgreSQL migrations
	@( \
		printf "Enter database name: "; read -r DB_NAME && \
		printf "Enter pass for db: "; read -rs PG_PASSWORD && \
		printf "Enter port(5432...): "; read -r PG_PORT && \
		PG_PORT=$${PG_PORT:-5432} && \
		PG_URI="postgres://root:$${PG_PASSWORD}@localhost:$${PG_PORT}/$${DB_NAME}?sslmode=disable" && \
		goose postgres "$${PG_URI}" -dir ${POSTGRES_SQL_PATH} down \
	)

# -----------------------------------------------------------------------------
# General
# -----------------------------------------------------------------------------

gen-migrate-sql: ## generate migration SQL files
	@mkdir -p ${MYSQL_SQL_PATH}
	@( \
		printf "Enter migration name: "; read -r MIGRATE_NAME && \
		migrate create -ext sql -dir ${MYSQL_SQL_PATH} $${MIGRATE_NAME} \
	)

###########
#   GCI   #
###########

GCI_DOMAIN_PREFIX ?=

gci-format: ## format imports
	gci write --skip-generated -s standard -s default $$(if $(GCI_DOMAIN_PREFIX),-s "prefix($${GCI_DOMAIN_PREFIX})",) -s "prefix($(PROJECT_NAME))" ./

#########
# build #
#########

build: ## build the project
	@( \
		printf "Enter version: "; read -r VERSION; \
		go build -ldflags "-s -w -X 'main.Version=$$VERSION' -X 'main.Built=$(Date)' -X 'main.GitCommit=$(GitCommit)'" -o ./bin/$(PROJECT_NAME) ./cmd/$(PROJECT_NAME) \
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
		--proto_path=$$(go env GOPATH)/pkg/mod/google.golang.org/protobuf@v1.36.2/src \
		--go_out=proto/pb/authpb \
		--go_opt=paths=source_relative \
		--go-grpc_out=proto/pb/authpb \
		--go-grpc_opt=paths=source_relative \
		--go_opt=default_api_level=API_OPAQUE \
		--experimental_allow_proto3_optional \
		proto/pb/auth.proto

proxy.gen: ## generate proxy code
	go run cmd/generator/main.go --config=.generator.yaml
