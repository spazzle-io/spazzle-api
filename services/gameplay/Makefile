mock:
	mockgen -package=mockdb --destination=internal/db/mock/store.go github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc Store
	mockgen -package=mockservices --destination=internal/services/mock/auth.go github.com/spazzle-io/spazzle-api/services/gameplay/internal/services AuthGrpcService
	mockgen -package=mockwordstore --destination=internal/wordstore/mock/wordstore.go github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore Store

.PHONY: mock
