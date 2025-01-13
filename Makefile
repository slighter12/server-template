.PHONY: help test-race lint sec-scan gci-format db-mysql-init build docker-image-build db-mysql-down db-mysql-up db-mysql-seeders-init gen-migrate-sql proto.gen

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
MYSQL_SEEDERS_SQL_PATH := ./database/migrations/seeders

# Automatically create migration directories if they do not exist
db-mysql-seeders-init: ## initialize new seeder
	@mkdir -p ${MYSQL_SEEDERS_SQL_PATH}
	@( \
	printf "Enter seeder name: "; read -r SEEDER_NAME && \
	touch ${MYSQL_SEEDERS_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$${SEEDER_NAME}.up.sql && \
	touch ${MYSQL_SEEDERS_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$${SEEDER_NAME}.down.sql \
	)

db-mysql-init: ## initialize new migration
	@mkdir -p ${MYSQL_SQL_PATH}
	@( \
	printf "Enter migrate name: "; read -r MIGRATE_NAME && \
	migrate create -ext sql -dir ${MYSQL_SQL_PATH} $${MIGRATE_NAME} \
	)

db-mysql-up: ## apply all migrations, including seeders
	@( \
	printf "Enter pass for db: "; read -rs DB_PASSWORD && \
	printf "Enter port(3306...): "; read -r DB_PORT &&\
	migrate --database "mysql://root:$${DB_PASSWORD}@tcp(localhost:$${DB_PORT})/$(PROJECT_NAME)?charset=utf8&parseTime=True&loc=Local" --path ${MYSQL_SQL_PATH} up && \
	migrate --database "mysql://root:$${DB_PASSWORD}@tcp(localhost:$${DB_PORT})/$(PROJECT_NAME)?charset=utf8&parseTime=True&loc=Local" --path ${MYSQL_SEEDERS_SQL_PATH} up \
	)

db-mysql-down: ## revert all migrations, including seeders
	@( \
	printf "Enter pass for db: "; read -rs DB_PASSWORD && \
	printf "Enter port(3306...): "; read -r DB_PORT &&\
	migrate --database "mysql://root:$${DB_PASSWORD}@tcp(localhost:$${DB_PORT})/$(PROJECT_NAME)?charset=utf8&parseTime=True&loc=Local" --path ${MYSQL_SEEDERS_SQL_PATH} down && \
	migrate --database "mysql://root:$${DB_PASSWORD}@tcp(localhost:$${DB_PORT})/$(PROJECT_NAME)?charset=utf8&parseTime=True&loc=Local" --path ${MYSQL_SQL_PATH} down \
	)

gen-migrate-sql: ## generate migration SQL files
	@mkdir -p ${MYSQL_SQL_PATH}
	@( \
	printf "Enter file name: "; read -r FILE_NAME; \
	touch ${MYSQL_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$$FILE_NAME.up.sql; \
	touch ${MYSQL_SQL_PATH}/$(SQL_FILE_TIMESTAMP)_$$FILE_NAME.down.sql; \
	)

###########
#   GCI   #
###########
gci-format: ## format imports
	gci write --skip-generated -s standard -s default -s "prefix(yt.com/backend)" -s "prefix($(PROJECT_NAME))" ./

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
		--ssh default=$$HOME/.ssh/id_rsa \
		./

DOCKER_PLATFORM ?= linux/amd64

PROTO_DIR := proto/pb
PROTO_GOOGLE := proto
PROTO_GEN_DIR := proto/pb/gen

proto.gen: ## generate protobuf code
	for file in $$(find ${PROTO_DIR} -name *.proto); do \
		protoc --proto_path=${PROTO_DIR} \
			--proto_path=${PROTO_GOOGLE} \
			--go_out=${PROTO_GEN_DIR} \
			--go_opt=paths=source_relative \
			--go-grpc_out=${PROTO_GEN_DIR} \
			--go-grpc_opt=paths=source_relative \
			--go_opt=default_api_level=API_OPAQUE \
			--experimental_allow_proto3_optional \
			$${file}; \
	done
