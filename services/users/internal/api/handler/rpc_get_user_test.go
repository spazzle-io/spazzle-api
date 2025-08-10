package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	mockdb "github.com/spazzle-io/spazzle-api/services/users/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	mockservices "github.com/spazzle-io/spazzle-api/services/users/internal/services/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetUser(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.GetUserRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.GetUserResponse, err error)
	}{
		{
			name: "success",
			req: &pb.GetUserRequest{
				Id: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.User{
						ID:            uuid.New(),
						WalletAddress: "some_address",
						GamerTag: pgtype.Text{
							String: "some_gamer_tag",
							Valid:  true,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetUserResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.User)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.GetUserRequest{
				Id: "fake_id",
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.GetUserResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"id"}
				checkInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not find user",
			req: &pb.GetUserRequest{
				Id: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.User{}, errors.New("user not found"))
			},
			checkResponse: func(t *testing.T, res *pb.GetUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UserNotFoundError)
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

			res, err := handler.GetUser(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
