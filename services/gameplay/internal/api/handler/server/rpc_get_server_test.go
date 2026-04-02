package server

import (
	"context"
	"math/big"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetServer(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.GetServerRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.GetServerResponse, err error)
	}{
		{
			name: "success",
			req: &pb.GetServerRequest{
				Id: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID: uuid.New(),
						StakePerGame: pgtype.Numeric{
							Int:   big.NewInt(1),
							Valid: true,
						},
						TotalVolume: pgtype.Numeric{
							Int:   big.NewInt(1000),
							Valid: true,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetServerResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.GetServerRequest{
				Id: "fake_id",
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.GetServerResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedViolations := []string{"id"}
				handler.CheckInvalidRequestParams(t, err, expectedViolations)
			},
		},
		{
			name: "could not get server",
			req: &pb.GetServerRequest{
				Id: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, db.RecordNotFoundError)
			},
			checkResponse: func(t *testing.T, res *pb.GetServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.ServerNotFoundError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not map db server to pb",
			req: &pb.GetServerRequest{
				Id: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID: uuid.New(),
						StakePerGame: pgtype.Numeric{
							Valid: false,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetServerResponse, err error) {
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

			res, err := h.GetServer(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
