mock:
	mockgen -package=mockdb --destination=internal/db/mock/store.go github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc Store
	mockgen -package mockwk -destination=internal/worker/mock/distributor.go github.com/spazzle-io/spazzle-api/services/users/internal/worker TaskDistributor
	mockgen -package mockwk -destination=internal/worker/mock/processor.go github.com/spazzle-io/spazzle-api/services/users/internal/worker TaskProcessor

.PHONY: mock
