verify:
	workflowcheck github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/workflow

mock:
	mockgen -package=mockdb --destination=internal/db/mock/store.go github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc Store
	mockgen -package=mockservices --destination=internal/services/mock/auth.go github.com/spazzle-io/spazzle-api/services/gameplay/internal/services AuthGrpcService
	mockgen -package=mockwordstore --destination=internal/wordstore/mock/wordstore.go github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore Store
	mockgen -package=mockgameflow --destination=internal/gameflow/mock/temporal.go -mock_names Client=MockTemporal go.temporal.io/sdk/client Client
	mockgen -package=mockgameflow --destination=internal/gameflow/mock/client.go github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow Client
	mockgen -package=mockeventbus --destination=internal/eventbus/mock/eventbus.go github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus EventBus
	mockgen -package=mockeventbus --destination=internal/eventbus/mock/session.go github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus Session
	mockgen -package=mockworker --destination=internal/worker/mock/distributor.go github.com/spazzle-io/spazzle-api/services/gameplay/internal/worker TaskDistributor
	mockgen -package=mocktreasury --destination=internal/treasury/mock/safekit_client.go -mock_names Client=SafeClient github.com/spazzle-io/safekit/pkg/safe Client
	mockgen -package=mocktreasury -destination=internal/treasury/mock/client.go github.com/spazzle-io/spazzle-api/services/gameplay/internal/treasury Client

.PHONY: verify mock
