package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/spazzle-io/safekit/pkg/safe"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/treasury"
	mocktreasury "github.com/spazzle-io/spazzle-api/services/gameplay/internal/treasury/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskDeployTreasury(t *testing.T) {
	testServerID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	testOwner := common.HexToAddress("0x1111111111111111111111111111111111111111")
	testAddress := "0x2222222222222222222222222222222222222222"

	testServer := db.Server{
		ID:            testServerID,
		ServerAddress: testAddress,
	}

	testTreasury := db.ServerTreasury{
		Address: testAddress,
		Status:  db.TreasuryStatusPending,
	}

	testDeployResult := &treasury.DeployResult{
		Address:     common.HexToAddress(testAddress),
		TxHash:      common.HexToHash("0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
		BlockNumber: 100,
		GasUsed:     283510,
	}

	validPayload := func() []byte {
		b, _ := json.Marshal(PayloadDeployTreasury{
			ServerID:     testServerID,
			OwnerAddress: testOwner,
		})
		return b
	}

	testCases := []struct {
		name          string
		payload       []byte
		buildStubs    func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient)
		checkResponse func(t *testing.T, err error)
	}{
		{
			name:    "success",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.HexToAddress(testAddress), nil)

				treasuryClient.EXPECT().
					IsDeployed(gomock.Any(), testServerID, testOwner).
					Return(false, nil)

				store.EXPECT().
					MarkTreasuryDeploying(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					Deploy(gomock.Any(), testServerID, testOwner).
					Return(testDeployResult, nil)

				store.EXPECT().
					MarkTreasuryDeployed(gomock.Any(), gomock.Any()).
					Return(testTreasury, nil)
			},
			checkResponse: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:    "invalid payload",
			payload: []byte("not json"),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().GetServerById(gomock.Any(), gomock.Any()).Times(0)
				treasuryClient.EXPECT().PredictAddress(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "server not found",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(db.Server{}, db.RecordNotFoundError)

				treasuryClient.EXPECT().PredictAddress(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "get server db error",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(db.Server{}, errors.New("connection refused"))

				treasuryClient.EXPECT().PredictAddress(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "treasury not found",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(db.ServerTreasury{}, db.RecordNotFoundError)

				treasuryClient.EXPECT().PredictAddress(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "get treasury db error",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(db.ServerTreasury{}, errors.New("connection refused"))

				treasuryClient.EXPECT().PredictAddress(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "already deployed in db",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(db.ServerTreasury{
						Address: testAddress,
						Status:  db.TreasuryStatusDeployed,
					}, nil)

				treasuryClient.EXPECT().PredictAddress(gomock.Any(), gomock.Any()).Times(0)
				treasuryClient.EXPECT().Deploy(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:    "predict address error",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.Address{}, errors.New("rpc error"))

				treasuryClient.EXPECT().Deploy(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "address mismatch",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.HexToAddress("0x9999999999999999999999999999999999999999"), nil)

				store.EXPECT().
					MarkTreasuryFailed(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().Deploy(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "is deployed check error",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.HexToAddress(testAddress), nil)

				// failed deployment check does not prevent a deployment attempt
				treasuryClient.EXPECT().
					IsDeployed(gomock.Any(), testServerID, testOwner).
					Return(false, errors.New("rpc error"))

				store.EXPECT().
					MarkTreasuryDeploying(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					Deploy(gomock.Any(), testServerID, testOwner).
					Return(testDeployResult, nil)

				store.EXPECT().
					MarkTreasuryDeployed(gomock.Any(), gomock.Any()).
					Return(testTreasury, nil)
			},
			checkResponse: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:    "already deployed on-chain recovery",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.HexToAddress(testAddress), nil)

				treasuryClient.EXPECT().
					IsDeployed(gomock.Any(), testServerID, testOwner).
					Return(true, nil)

				store.EXPECT().
					RecoverDeployedTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().Deploy(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:    "recover deployed treasury db error",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.HexToAddress(testAddress), nil)

				treasuryClient.EXPECT().
					IsDeployed(gomock.Any(), testServerID, testOwner).
					Return(true, nil)

				store.EXPECT().
					RecoverDeployedTreasury(gomock.Any(), testAddress).
					Return(db.ServerTreasury{}, errors.New("connection refused"))

				treasuryClient.EXPECT().Deploy(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "mark treasury deploying db error",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.HexToAddress(testAddress), nil)

				treasuryClient.EXPECT().
					IsDeployed(gomock.Any(), testServerID, testOwner).
					Return(false, nil)

				store.EXPECT().
					MarkTreasuryDeploying(gomock.Any(), testAddress).
					Return(db.ServerTreasury{}, errors.New("connection refused"))

				treasuryClient.EXPECT().Deploy(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "deploy timeout",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.HexToAddress(testAddress), nil)

				treasuryClient.EXPECT().
					IsDeployed(gomock.Any(), testServerID, testOwner).
					Return(false, nil)

				store.EXPECT().
					MarkTreasuryDeploying(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					Deploy(gomock.Any(), testServerID, testOwner).
					Return(nil, fmt.Errorf("treasury: %w", safe.ErrDeployTimeout))

				store.EXPECT().MarkTreasuryDeployed(gomock.Any(), gomock.Any()).Times(0)
				store.EXPECT().MarkTreasuryFailed(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "transaction reverted",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.HexToAddress(testAddress), nil)

				treasuryClient.EXPECT().
					IsDeployed(gomock.Any(), testServerID, testOwner).
					Return(false, nil)

				store.EXPECT().
					MarkTreasuryDeploying(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					Deploy(gomock.Any(), testServerID, testOwner).
					Return(nil, fmt.Errorf("treasury: %w", safe.ErrTransactionReverted))

				store.EXPECT().
					MarkTreasuryFailed(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				store.EXPECT().MarkTreasuryDeployed(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "deployment mismatch error",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.HexToAddress(testAddress), nil)

				treasuryClient.EXPECT().
					IsDeployed(gomock.Any(), testServerID, testOwner).
					Return(false, nil)

				store.EXPECT().
					MarkTreasuryDeploying(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					Deploy(gomock.Any(), testServerID, testOwner).
					Return(nil, &safe.DeploymentMismatchError{
						PredictedAddress: common.HexToAddress(testAddress),
						ActualAddress:    common.HexToAddress("0x9999999999999999999999999999999999999999"),
					})

				store.EXPECT().
					MarkTreasuryFailed(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				store.EXPECT().MarkTreasuryDeployed(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "deploy generic error",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.HexToAddress(testAddress), nil)

				treasuryClient.EXPECT().
					IsDeployed(gomock.Any(), testServerID, testOwner).
					Return(false, nil)

				store.EXPECT().
					MarkTreasuryDeploying(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					Deploy(gomock.Any(), testServerID, testOwner).
					Return(nil, errors.New("rpc unavailable"))

				store.EXPECT().
					MarkTreasuryFailed(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				store.EXPECT().MarkTreasuryDeployed(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name:    "mark deployed db error. Recovers on retry",
			payload: validPayload(),
			buildStubs: func(store *mockdb.MockStore, treasuryClient *mocktreasury.MockClient) {
				store.EXPECT().
					GetServerById(gomock.Any(), testServerID).
					Return(testServer, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					PredictAddress(testServerID, testOwner).
					Return(common.HexToAddress(testAddress), nil)

				treasuryClient.EXPECT().
					IsDeployed(gomock.Any(), testServerID, testOwner).
					Return(false, nil)

				store.EXPECT().
					MarkTreasuryDeploying(gomock.Any(), testAddress).
					Return(testTreasury, nil)

				treasuryClient.EXPECT().
					Deploy(gomock.Any(), testServerID, testOwner).
					Return(testDeployResult, nil)

				store.EXPECT().
					MarkTreasuryDeployed(gomock.Any(), gomock.Any()).
					Return(db.ServerTreasury{}, errors.New("connection refused"))
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			treasuryClient := mocktreasury.NewMockClient(ctrl)

			processor := &RedisTaskProcessor{
				store:          store,
				treasuryClient: treasuryClient,
			}

			tc.buildStubs(store, treasuryClient)

			task := asynq.NewTask(TaskDeployTreasury, tc.payload)
			err := processor.processTaskDeployTreasury(context.Background(), task)
			tc.checkResponse(t, err)
		})
	}
}
