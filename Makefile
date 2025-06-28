# Whether to run short tests
SHORT ?= true
# Define the service to run the target against
SERVICE ?=
# Dynamically determine all services
SERVICES := $(shell find services -maxdepth 1 -type d -not -path 'services')

test:
ifdef SERVICE
	@echo "Running tests for service: $(SERVICE)"
	go test -v -race -cover -coverprofile=coverage.out -covermode=atomic -short=$(SHORT) ./services/$(SERVICE)/...
else
	@echo "Running tests for all services"
	@for svc in $(SERVICES); do \
		echo "Testing $$svc..."; \
		go test -v -race -cover -coverprofile=coverage.out -covermode=atomic -short=$(SHORT) ./$$svc/... || exit 1; \
	done
endif

.PHONY: test
