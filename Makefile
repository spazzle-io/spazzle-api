# Whether to run short tests
short ?= true
# Define the module to run the target against
# A module is either a service e.g "services/auth" or a library e.g "libs/common"
module ?=
# Dynamically determine all modules
modules := $(shell find services libs -mindepth 1 -maxdepth 1 -type d)

POSTGRES_USER := root
POSTGRES_PORT := 5432
POSTGRES_PASS := password
POSTGRES_DB_NAME := $(notdir $(module)) # Set the DB name as the directory name without the path prefix e.g "auth" for module: "services/auth"
POSTGRES_VERSION := 17-alpine

REDIS_PORT := 6379
REDIS_VERSION := 8-alpine

define compute-db-url
	db_name=$$(basename $(1)); \
	db_url="postgresql://$(POSTGRES_USER):$(POSTGRES_PASS)@localhost:$(POSTGRES_PORT)/$$db_name?sslmode=disable"
endef

run-module-target:
	@if [ -f ./$(module)/Makefile ]; then \
		if grep -qE '^[^#]*\<$(target)\>[: ]' ./$(module)/Makefile; then \
			echo "Running $(target) in $(module)"; \
			$(MAKE) -C ./$(module) $(target); \
		else \
			echo "$(target) target not found in $(module)"; \
		fi; \
	else \
		echo "Makefile not found in $(module)"; \
	fi

test:
ifdef module
	@echo "Running tests for module: $(module)"
	@coverprofile_base_name=$(subst /,-,$(module))-coverage.out; \
	count_flag=; \
    if [ "$(short)" = "false" ]; then count_flag="-count=1"; fi; \
	go test -v -race -cover -coverprofile=$$coverprofile_base_name -covermode=atomic -short=$(short) $$count_flag ./$(module)/... || exit 1; \
	$(MAKE) run-module-target module=$(module) target=verify || exit 1;
else
	@echo "Running tests for all modules"
	@count_flag=; \
    if [ "$(short)" = "false" ]; then count_flag="-count=1"; fi; \
	for mod in $(modules); do \
		echo "Testing $$mod..."; \
		coverprofile_base_name=$$(echo $$mod | tr '/' '-')-coverage.out; \
		go test -v -race -cover -coverprofile=$$coverprofile_base_name -covermode=atomic -short=$(short) $$count_flag ./$$mod/... || exit 1; \
		$(MAKE) run-module-target module=$$mod target=verify || exit 1; \
	done
endif
	@$(MAKE) merge-coverage

merge-coverage:
	@echo "Merging coverage reports..."
	@echo "mode: atomic" > combined-coverage.out
	@tail -q -n +2 *-coverage.out >> combined-coverage.out
	@rm -f $(filter-out combined-coverage.out,$(wildcard *-coverage.out))

tidy:
	@echo "Running go mod tidy for all modules"
	@for mod in $(modules); do \
		echo "Tidying up $$mod..."; \
		( cd ./$$mod && go mod tidy ) || exit 1; \
	done

postgres:
	docker run --name postgres$(POSTGRES_VERSION) -p $(POSTGRES_PORT):$(POSTGRES_PORT) -e POSTGRES_USER=$(POSTGRES_USER) -e POSTGRES_PASSWORD=$(POSTGRES_PASS) -d postgres:$(POSTGRES_VERSION)

create_db:
	docker exec -it postgres$(POSTGRES_VERSION) createdb --username=$(POSTGRES_USER) --owner=$(POSTGRES_USER) $(POSTGRES_DB_NAME)

drop_db:
	docker exec -it postgres$(POSTGRES_VERSION) dropdb $(POSTGRES_DB_NAME)

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
	docker run --name redis$(REDIS_VERSION) -p $(REDIS_PORT):$(REDIS_PORT) -d redis:$(REDIS_VERSION)

mock:
	@$(MAKE) run-module-target module=$(module) target=mock

sqlc:
	cd ./$(module) && sqlc generate
	@$(MAKE) mock

server:
	cd ./$(module) && go run ./cmd/$(notdir $(module))

buf_update:
	@cd ./$(module)/api && buf dep update && buf lint && buf build && buf breaking --against "https://github.com/spazzle-io/spazzle-api.git#branch=main,subdir=$(module)/api/proto"

proto:
	@rm -f ./libs/common/docs/swagger/$(notdir $(module)).swagger.json
	@rm -rf ./services/proto/$(notdir $(module))
	@rm -rf ./libs/common/docs/statik
	@cd ./$(module)/api && buf lint && buf build && buf generate && buf breaking --against "https://github.com/spazzle-io/spazzle-api.git#branch=main,subdir=$(module)/api/proto"
	@statik -src=./libs/common/docs/swagger -dest=./libs/common/docs
	@cd ./services/proto && go install tool && go mod tidy

temporal-up:
	docker compose -f docker/temporal/docker-compose.temporal.yml up -d

temporal-down:
	docker compose -f docker/temporal/docker-compose.temporal.yml down --volumes

temporal-logs:
	docker compose -f docker/temporal/docker-compose.temporal.yml logs -f

minio-up:
	docker compose -f docker/minio/docker-compose.minio.yml up -d

minio-down:
	docker compose -f docker/minio/docker-compose.minio.yml down --volumes

minio-logs:
	docker compose -f docker/minio/docker-compose.minio.yml logs -f

.PHONY: run-module-target test merge-coverage tidy postgres create_db drop_db migrate_create migrate_up migrate_down redis mock sqlc buf_update proto temporal-up temporal-down temporal-logs minio-up minio-down minio-logs
