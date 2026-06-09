package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/protobuf/ptypes/wrappers"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spazzle-io/spazzle-api/libs/common/util"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	mockdb "github.com/spazzle-io/spazzle-api/services/users/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
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

func TestListUsers(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.ListUsersRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.ListUsersResponse, err error)
	}{
		{
			name: "success",
			req: &pb.ListUsersRequest{
				PageSize: &wrappers.Int32Value{
					Value: defaultPageSize,
				},
			},
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
				require.NotEmpty(t, res.Users)
				require.NotEmpty(t, res.Cursor)
				require.Equal(t, int64(2), res.TotalCount)
				require.Len(t, res.GetUsers(), 2)
			},
		},
		{
			name: "page size is zero",
			req: &pb.ListUsersRequest{
				PageSize: &wrappers.Int32Value{
					Value: 0,
				},
			},
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
				require.Equal(t, res.Cursor.PageSize, defaultPageSize)
			},
		},
		{
			name: "page size is greater than allowed maximum",
			req: &pb.ListUsersRequest{
				PageSize: &wrappers.Int32Value{
					Value: maxPageSize + 1,
				},
			},
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
				require.Equal(t, res.Cursor.PageSize, defaultPageSize)
			},
		},
		{
			name: "invalid after id",
			req: &pb.ListUsersRequest{
				PageSize: &wrappers.Int32Value{
					Value: defaultPageSize,
				},
				AfterId: &wrappers.StringValue{
					Value: "abc",
				},
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.ListUsersResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InvalidAfterIdError)
				require.Empty(t, res)
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
			deps := newTestDeps(t)

			tc.buildStubs(deps.store)

			h := newTestHandler(deps)

			res, err := h.ListUsers(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
