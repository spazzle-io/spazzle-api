package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	mockdb "github.com/spazzle-io/spazzle-api/services/users/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	mockservices "github.com/spazzle-io/spazzle-api/services/users/internal/services/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func generateAuthenticateUserReqParams() *pb.AuthenticateUserRequest {
	return &pb.AuthenticateUserRequest{
		UserId:    uuid.New().String(),
		Signature: "some-signature",
	}
}

func TestAuthenticateUser(t *testing.T) {
	authenticateUserReqParams := generateAuthenticateUserReqParams()
	require.NotEmpty(t, authenticateUserReqParams)

	userId, err := uuid.Parse(authenticateUserReqParams.UserId)
	require.NoError(t, err)
	require.NotNil(t, userId)

	testCases := []struct {
		name          string
		req           *pb.AuthenticateUserRequest
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.AuthenticateUserResponse, err error)
	}{
		{
			name: "success",
			req:  authenticateUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				user := db.User{
					ID:            userId,
					WalletAddress: "0x4DeCC727221f50D8B341297dF43a11756bb27977",
					CreatedAt:     time.Now().UTC(),
				}

				store.EXPECT().
					GetUserById(gomock.Any(), userId).
					Times(1).
					Return(user, nil)

				authenticateReq := &authPb.AuthenticateRequest{
					WalletAddress: user.WalletAddress,
					UserId:        user.ID.String(),
					Signature:     authenticateUserReqParams.Signature,
				}

				authService.EXPECT().
					Authenticate(gomock.Any(), "test", authenticateReq).
					Times(1).
					Return(&authPb.AuthenticateResponse{
						Credential: &authPb.Credential{
							Id:            uuid.New().String(),
							UserId:        authenticateReq.UserId,
							WalletAddress: authenticateReq.GetWalletAddress(),
							CreatedAt:     timestamppb.New(time.Now().UTC()),
						},
						Session: &authPb.Session{
							SessionId: uuid.New().String(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateUserResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.NotEmpty(t, res.GetUser())
				require.NotEmpty(t, res.GetSession())

				require.Equal(t, authenticateUserReqParams.GetUserId(), res.GetUser().GetUserId())
			},
		},
		{
			name: "invalid request arguments",
			req: &pb.AuthenticateUserRequest{
				UserId:    "fake_user_id",
				Signature: "",
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, res *pb.AuthenticateUserResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"userId", "signature"}
				checkInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not get user by id",
			req:  authenticateUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetUserById(gomock.Any(), userId).
					Times(1).
					Return(db.User{}, errors.New("user not found"))
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UserNotFoundError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not authenticate user",
			req:  authenticateUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				user := db.User{
					ID:            userId,
					WalletAddress: "0x4DeCC727221f50D8B341297dF43a11756bb27977",
					CreatedAt:     time.Now().UTC(),
				}

				store.EXPECT().
					GetUserById(gomock.Any(), userId).
					Times(1).
					Return(user, nil)

				authenticateReq := &authPb.AuthenticateRequest{
					WalletAddress: user.WalletAddress,
					UserId:        user.ID.String(),
					Signature:     authenticateUserReqParams.Signature,
				}

				authService.EXPECT().
					Authenticate(gomock.Any(), "test", authenticateReq).
					Times(1).
					Return(&authPb.AuthenticateResponse{}, errors.New("could not authenticate user"))
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateUserResponse, err error) {
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

			tc.buildStubs(store, authService)

			handler := newTestHandler(store, cache, authService)

			res, err := handler.AuthenticateUser(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
