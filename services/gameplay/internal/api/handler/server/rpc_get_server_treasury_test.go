package server

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetServerTreasury(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.GetServerTreasuryRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.GetServerTreasuryResponse, err error)
	}{
		{
			name: "success",
			req: &pb.GetServerTreasuryRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ServerAddress: common.HexToAddress("0x123").Hex(),
					}, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), gomock.Eq(common.HexToAddress("0x123").Hex())).
					Times(1).
					Return(db.ServerTreasury{
						Address: common.HexToAddress("0x456").Hex(),
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetServerTreasuryResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.Treasury)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.GetServerTreasuryRequest{
				ServerId: "invalid-server-id",
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.GetServerTreasuryResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedViolations := []string{"serverId"}
				handler.CheckInvalidRequestParams(t, err, expectedViolations)
			},
		},
		{
			name: "failed to get server by id",
			req: &pb.GetServerTreasuryRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, errors.New("failed to get server by id"))
			},
			checkResponse: func(t *testing.T, res *pb.GetServerTreasuryResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "failed to get server treasury",
			req: &pb.GetServerTreasuryRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ServerAddress: common.HexToAddress("0x123").Hex(),
					}, nil)

				store.EXPECT().
					GetTreasury(gomock.Any(), gomock.Eq(common.HexToAddress("0x123").Hex())).
					Times(1).
					Return(db.ServerTreasury{}, errors.New("failed to get server treasury"))
			},
			checkResponse: func(t *testing.T, res *pb.GetServerTreasuryResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newTestDeps(t)

			tc.buildStubs(deps.store)

			h := newTestHandler(deps)

			res, err := h.GetServerTreasury(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
