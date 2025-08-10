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

func TestGetUserByWalletAddress(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.GetUserByWalletAddressRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.GetUserByWalletAddressResponse, err error)
	}{
		{
			name: "success",
			req: &pb.GetUserByWalletAddressRequest{
				WalletAddress: "0x4DeCC727221f50D8B341297dF43a11756bb27977",
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByWalletAddress(gomock.Any(), gomock.Any()).
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
			checkResponse: func(t *testing.T, res *pb.GetUserByWalletAddressResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res.User)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.GetUserByWalletAddressRequest{
				WalletAddress: "",
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.GetUserByWalletAddressResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"walletAddress", "walletAddress"}
				checkInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not find user",
			req: &pb.GetUserByWalletAddressRequest{
				WalletAddress: "0x4DeCC727221f50D8B341297dF43a11756bb27977",
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByWalletAddress(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.User{}, errors.New("user not found"))
			},
			checkResponse: func(t *testing.T, res *pb.GetUserByWalletAddressResponse, err error) {
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

			res, err := handler.GetUserByWalletAddress(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
