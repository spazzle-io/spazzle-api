# Whether to run short tests
short ?= true
# Define the module to run the target against
# A module is either a service e.g "services/auth" or a library e.g "libs/common"
module ?=
# Dynamically determine all modules
modules := $(shell find services libs -mindepth 1 -maxdepth 1 -type d)

DB_USER := root
DB_PORT := 5432
DB_PASS := password
DATABASE_NAME := $(notdir $(module)) # Set the DB name as the directory name without the path prefix e.g "auth" for module: "services/auth"
POSTGRES_VERSION := 17
DATABASE_URL := "postgresql://$(DB_USER):$(DB_PASS)@localhost:$(DB_PORT)/$(strip $(DATABASE_NAME))?sslmode=disable"

test:
ifdef module
	@echo "Running tests for module: $(module)"
	go test -v -race -cover -coverprofile=coverage.out -covermode=atomic -short=$(short) ./$(module)/...
else
	@echo "Running tests for all modules"
	@for mod in $(modules); do \
		echo "Testing $$mod..."; \
		go test -v -race -cover -coverprofile=coverage.out -covermode=atomic -short=$(short) ./$$mod/... || exit 1; \
	done
endif

db_schema:
	dbml2sql --postgres -o ./$(module)/docs/db_schema.sql ./$(module)/docs/db.dbml

postgres:
	docker run --name postgres$(POSTGRES_VERSION) -p $(DB_PORT):$(DB_PORT) -e POSTGRES_USER=$(DB_USER) -e POSTGRES_PASSWORD=$(DB_PASS) -d postgres:$(POSTGRES_VERSION)-alpine

create_db:
	docker exec -it postgres$(POSTGRES_VERSION) createdb --username=$(DB_USER) --owner=$(DB_USER) $(DATABASE_NAME)

drop_db:
	docker exec -it postgres$(POSTGRES_VERSION) dropdb $(DATABASE_NAME)

migrate_create:
	migrate create -ext sql -dir ./$(module)/internal/db/migration -seq $(name)

migrate_up:
ifdef n
	migrate -path ./$(module)/internal/db/migration -database $(DATABASE_URL) -verbose up $(n)
else
	migrate -path ./$(module)/internal/db/migration -database $(DATABASE_URL) -verbose up
endif

migrate_down:
ifdef n
	migrate -path ./$(module)/internal/db/migration -database $(DATABASE_URL) -verbose down $(n)
else
	migrate -path ./$(module)/internal/db/migration -database $(DATABASE_URL) -verbose down
endif

sqlc:
	cd ./$(module) && sqlc generate

.PHONY: test db_schema postgres create_db drop_db migrate_create migrate_up migrate_down sqlc
