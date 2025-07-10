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
REDIS_VERSION := 8

define compute-db-url
	db_name=$$(basename $(1)); \
	db_url="postgresql://$(DB_USER):$(DB_PASS)@localhost:$(DB_PORT)/$$db_name?sslmode=disable"
endef

test:
ifdef module
	@echo "Running tests for module: $(module)"
	@coverprofile_base_name=$(subst /,-,$(module))-coverage.out; \
	go test -v -race -cover -coverprofile=$$coverprofile_base_name -covermode=atomic -short=$(short) ./$(module)/...
else
	@echo "Running tests for all modules"
	@for mod in $(modules); do \
		echo "Testing $$mod..."; \
		coverprofile_base_name=$$(echo $$mod | tr '/' '-')-coverage.out; \
		go test -v -race -cover -coverprofile=$$coverprofile_base_name -covermode=atomic -short=$(short) ./$$mod/... || exit 1; \
	done
endif

tidy:
	@echo "Running go mod tidy for all modules"
	@for mod in $(modules); do \
		echo "Tidying up $$mod..."; \
		( cd ./$$mod && go mod tidy ) || exit 1; \
	done

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
ifdef module
	@echo "Running migrate_up for module: $(module)"
	@if [ -n "$(n)" ]; then \
		$(call compute-db-url,$$module); \
		migrate -path ./$(module)/internal/db/migration -database $$db_url -verbose up $(n); \
	else \
		$(call compute-db-url,$$module); \
		migrate -path ./$(module)/internal/db/migration -database $$db_url -verbose up; \
	fi
else
	@echo "Running migrate_up for all modules"
	@for mod in $(modules); do \
		if echo "$$mod" | grep -q '^services/'; then \
			if [ -d "./$$mod/internal/db/migration" ]; then \
				echo "Migrating up $$mod..."; \
				$(call compute-db-url,$$mod); \
				migrate -path ./$$mod/internal/db/migration -database $$db_url -verbose up || exit 1; \
			else \
				echo "Skipping $$mod (no migration directory)"; \
			fi \
		else \
			echo "Skipping $$mod (not a service)"; \
		fi \
	done
endif

migrate_down:
ifdef module
	@echo "Running migrate_down for module: $(module)"
	@if [ -n "$(n)" ]; then \
		$(call compute-db-url,$$module); \
		migrate -path ./$(module)/internal/db/migration -database $$db_url -verbose down $(n); \
	else \
		$(call compute-db-url,$$module); \
		migrate -path ./$(module)/internal/db/migration -database $$db_url -verbose down; \
	fi
else
	@echo "Running migrate_down for all modules"
	@for mod in $(modules); do \
		if echo "$$mod" | grep -q '^services/'; then \
			echo "Migrating down $$mod..."; \
				$(call compute-db-url,$$mod); \
				migrate -path ./$$mod/internal/db/migration -database $$db_url -verbose down || exit 1; \
		else \
			echo "Skipping $$mod (not a service)"; \
		fi \
	done
endif

redis:
	docker run --name redis -p 6379:6379 -d redis:$(REDIS_VERSION)-alpine

mock:
	@sh -c " \
		if [ -f ./$(module)/Makefile ]; then \
			if grep -qE '^[^#]*\<mock\>[: ]' ./$(module)/Makefile; then \
				echo 'Running make mock in $(module)'; \
				make -C ./$(module) mock; \
			else \
				echo 'mock target not found in $(module)/Makefile'; \
			fi; \
		else \
			echo 'Makefile not found in $(module)'; \
		fi"

sqlc:
	cd ./$(module) && sqlc generate
	@$(MAKE) mock

server:
	cd ./$(module) && go run ./cmd/$(notdir $(module))

proto:
	@rm -f ./services/proto/$(notdir $(module))/*.go
	@rm -f ./libs/common/docs/swagger/$(notdir $(module)).swagger.json
	@rm -rf ./libs/common/docs/statik
	@protoc --proto_path=./$(module)/api/proto --go_out=./services/proto/$(notdir $(module)) --go_opt=paths=source_relative \
	--go-grpc_out=./services/proto/$(notdir $(module)) --go-grpc_opt=paths=source_relative \
	--grpc-gateway_out=./services/proto/$(notdir $(module)) --grpc-gateway_opt=paths=source_relative \
	--openapiv2_out=./libs/common/docs/swagger --openapiv2_opt=allow_merge=true,merge_file_name=$(notdir $(module)) \
	./$(module)/api/proto/*.proto
	@statik -src=./libs/common/docs/swagger -dest=./libs/common/docs
	@cd ./services/proto && go install tool && go mod tidy

.PHONY: test tidy db_schema postgres create_db drop_db migrate_create migrate_up migrate_down redis mock sqlc proto
