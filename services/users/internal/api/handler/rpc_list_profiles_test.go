package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/jackc/pgx/v5/pgtype"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/spazzle-io/spazzle-api/libs/common/util"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	mockdb "github.com/spazzle-io/spazzle-api/services/users/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	mockservices "github.com/spazzle-io/spazzle-api/services/users/internal/services/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func generateUsers(t *testing.T, numUsersToGenerate int) (dbUsers []db.User) {
	for i := 0; i < numUsersToGenerate; i++ {
		wallet, err := util.NewEthereumWallet()
		require.NoError(t, err)
		require.NotEmpty(t, wallet)

		dbUsers = append(dbUsers, db.User{
			WalletAddress: wallet.Address,
			GamerTag: pgtype.Text{
				String: gofakeit.Gamertag(),
				Valid:  true,
			},
		})
	}

	return
}

func TestListProfiles(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.ListUsersRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.ListUsersResponse, err error)
	}{
		{
			name: "success",
			req:  &pb.ListUsersRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return(generateUsers(t, 2), nil)

				store.EXPECT().
					GetTotalUserCount(gomock.Any()).
					Times(1).
					Return(int64(2), nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListUsersResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.Len(t, res.GetUsers(), 2)
				require.Equal(t, int64(2), res.GetNumTotalUsers())
				require.Equal(t, int32(1), res.GetPage())
				require.Equal(t, int32(30), res.GetPageSize())
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.ListUsersRequest{
				Page:     1,
				PageSize: 101,
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.ListUsersResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"pageSize"}
				checkInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not list users",
			req:  &pb.ListUsersRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.User{}, errors.New("could not list users"))
			},
			checkResponse: func(t *testing.T, res *pb.ListUsersResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not get total user count",
			req:  &pb.ListUsersRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return(generateUsers(t, 2), nil)

				store.EXPECT().
					GetTotalUserCount(gomock.Any()).
					Times(1).
					Return(int64(0), errors.New("could not get total user count"))
			},
			checkResponse: func(t *testing.T, res *pb.ListUsersResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
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

			handler := newTestHandler(store, cache, authService)

			res, err := handler.ListUsers(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
