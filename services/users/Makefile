mock:
	mockgen -package=mockdb --destination=internal/db/mock/store.go github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc Store
	mockgen -package=mockservices --destination=internal/services/mock/auth.go github.com/spazzle-io/spazzle-api/services/users/internal/services AuthGrpcService

.PHONY: mock
