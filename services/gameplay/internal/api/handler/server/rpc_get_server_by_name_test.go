package server

import (
	"context"
	"math/big"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	mockservices "github.com/spazzle-io/spazzle-api/services/gameplay/internal/services/mock"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetServerByName(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.GetServerByNameRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.GetServerByNameResponse, err error)
	}{
		{
			name: "success",
			req: &pb.GetServerByNameRequest{
				Name: "test",
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerByName(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID: uuid.New(),
						StakePerGame: pgtype.Numeric{
							Int:   big.NewInt(1),
							Valid: true,
						},
						TotalVolume: pgtype.Numeric{
							Int:   big.NewInt(1),
							Valid: true,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetServerByNameResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.Server)
			},
		},
		{
			name:       "invalid request parameters",
			req:        &pb.GetServerByNameRequest{},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.GetServerByNameResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedViolations := []string{"name"}
				handler.CheckInvalidRequestParams(t, err, expectedViolations)
			},
		},
		{
			name: "could not get server",
			req: &pb.GetServerByNameRequest{
				Name: "test",
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerByName(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, db.RecordNotFoundError)
			},
			checkResponse: func(t *testing.T, res *pb.GetServerByNameResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.ServerNotFoundError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not map db server to pb",
			req: &pb.GetServerByNameRequest{
				Name: "test",
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerByName(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID: uuid.New(),
						StakePerGame: pgtype.Numeric{
							Valid: false,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetServerByNameResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			cache := mockcache.NewMockCache(ctrl)
			authService := mockservices.NewMockAuthGrpcService(ctrl)

			tc.buildStubs(store)

			h := newTestHandler(store, cache, authService)

			res, err := h.GetServerByName(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
