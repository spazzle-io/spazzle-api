package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"

	"github.com/google/uuid"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	mockdb "github.com/spazzle-io/spazzle-api/services/users/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	mockservices "github.com/spazzle-io/spazzle-api/services/users/internal/services/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func generateAuthenticateUserReqParams(t *testing.T) *pb.AuthenticateUserRequest {
	wallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, wallet)

	return &pb.AuthenticateUserRequest{
		WalletAddress: wallet.Address,
		Signature:     "some-signature",
	}
}

func TestAuthenticateUser(t *testing.T) {
	authenticateUserReqParams := generateAuthenticateUserReqParams(t)
	require.NotEmpty(t, authenticateUserReqParams)

	testCases := []struct {
		name          string
		req           *pb.AuthenticateUserRequest
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.AuthenticateUserResponse, err error)
	}{
		{
			name: "success - existing user",
			req:  authenticateUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				user := db.User{
					ID:            uuid.New(),
					WalletAddress: authenticateUserReqParams.GetWalletAddress(),
					CreatedAt:     time.Now().UTC(),
				}

				store.EXPECT().
					GetUserByWalletAddress(gomock.Any(), authenticateUserReqParams.GetWalletAddress()).
					Times(1).
					Return(user, nil)

				authenticateReq := &authPb.AuthenticateRequest{
					WalletAddress: user.WalletAddress,
					UserId:        user.ID.String(),
					Signature:     authenticateUserReqParams.Signature,
				}

				authService.EXPECT().
					Authenticate(gomock.Any(), gomock.Any(), authenticateReq).
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

				require.Equal(t, authenticateUserReqParams.GetWalletAddress(), res.GetUser().GetWalletAddress())
			},
		},
		{
			name: "success - new user",
			req:  authenticateUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				createdUser := db.User{
					ID:            uuid.New(),
					WalletAddress: authenticateUserReqParams.GetWalletAddress(),
					CreatedAt:     time.Now().UTC(),
				}

				store.EXPECT().
					GetUserByWalletAddress(gomock.Any(), authenticateUserReqParams.GetWalletAddress()).
					Times(1).
					Return(db.User{}, nil)

				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CreateUserTxParams) (db.CreateUserTxResult, error) {
						err := arg.AfterCreate(createdUser)
						if err != nil {
							return db.CreateUserTxResult{}, err
						}

						return db.CreateUserTxResult{
							User: createdUser,
						}, nil
					})

				authenticateReq := &authPb.AuthenticateRequest{
					WalletAddress: createdUser.WalletAddress,
					UserId:        createdUser.ID.String(),
					Signature:     authenticateUserReqParams.Signature,
				}

				authService.EXPECT().
					Authenticate(gomock.Any(), gomock.Any(), authenticateReq).
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

				require.Equal(t, authenticateUserReqParams.GetWalletAddress(), res.GetUser().GetWalletAddress())
			},
		},
		{
			name: "invalid request arguments",
			req: &pb.AuthenticateUserRequest{
				WalletAddress: "",
				Signature:     "",
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, res *pb.AuthenticateUserResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"walletAddress", "walletAddress", "signature"}
				checkInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not get user by wallet address",
			req:  authenticateUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetUserByWalletAddress(gomock.Any(), authenticateUserReqParams.GetWalletAddress()).
					Times(1).
					Return(db.User{}, errors.New("user not found"))
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not authenticate user - existing user",
			req:  authenticateUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				user := db.User{
					ID:            uuid.New(),
					WalletAddress: authenticateUserReqParams.GetWalletAddress(),
					CreatedAt:     time.Now().UTC(),
				}

				store.EXPECT().
					GetUserByWalletAddress(gomock.Any(), authenticateUserReqParams.GetWalletAddress()).
					Times(1).
					Return(user, nil)

				authenticateReq := &authPb.AuthenticateRequest{
					WalletAddress: user.WalletAddress,
					UserId:        user.ID.String(),
					Signature:     authenticateUserReqParams.Signature,
				}

				authService.EXPECT().
					Authenticate(gomock.Any(), gomock.Any(), authenticateReq).
					Times(1).
					Return(&authPb.AuthenticateResponse{}, errors.New("could not authenticate user"))
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not authenticate user - new user",
			req:  authenticateUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				createdUser := db.User{
					ID:            uuid.New(),
					WalletAddress: authenticateUserReqParams.GetWalletAddress(),
					CreatedAt:     time.Now().UTC(),
				}

				store.EXPECT().
					GetUserByWalletAddress(gomock.Any(), authenticateUserReqParams.GetWalletAddress()).
					Times(1).
					Return(db.User{}, nil)

				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CreateUserTxParams) (db.CreateUserTxResult, error) {
						err := arg.AfterCreate(createdUser)
						if err != nil {
							return db.CreateUserTxResult{}, err
						}

						return db.CreateUserTxResult{
							User: createdUser,
						}, nil
					})

				authenticateReq := &authPb.AuthenticateRequest{
					WalletAddress: createdUser.WalletAddress,
					UserId:        createdUser.ID.String(),
					Signature:     authenticateUserReqParams.Signature,
				}

				authService.EXPECT().
					Authenticate(gomock.Any(), gomock.Any(), authenticateReq).
					Times(1).
					Return(&authPb.AuthenticateResponse{}, errors.New("could not authenticate user"))
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not authenticate user - new user - user already exists",
			req:  authenticateUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				createdUser := db.User{
					ID:            uuid.New(),
					WalletAddress: authenticateUserReqParams.GetWalletAddress(),
					CreatedAt:     time.Now().UTC(),
				}

				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CreateUserTxParams) (db.CreateUserTxResult, error) {
						err := arg.AfterCreate(createdUser)
						if err != nil {
							return db.CreateUserTxResult{}, err
						}

						return db.CreateUserTxResult{
							User: createdUser,
						}, nil
					})

				store.EXPECT().
					GetUserByWalletAddress(gomock.Any(), authenticateUserReqParams.GetWalletAddress()).
					Times(1).
					Return(db.User{}, nil)

				authenticateReq := &authPb.AuthenticateRequest{
					WalletAddress: createdUser.WalletAddress,
					UserId:        createdUser.ID.String(),
					Signature:     authenticateUserReqParams.Signature,
				}

				authService.EXPECT().
					Authenticate(gomock.Any(), gomock.Any(), authenticateReq).
					Times(1).
					Return(&authPb.AuthenticateResponse{}, db.ErrUserAlreadyExists)
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UserAlreadyExists)
				require.Empty(t, res)
			},
		},
		{
			name: "could not authenticate user - new user - gamer tag already in use",
			req:  authenticateUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				createdUser := db.User{
					ID:            uuid.New(),
					WalletAddress: authenticateUserReqParams.GetWalletAddress(),
					CreatedAt:     time.Now().UTC(),
				}

				store.EXPECT().
					GetUserByWalletAddress(gomock.Any(), authenticateUserReqParams.GetWalletAddress()).
					Times(1).
					Return(db.User{}, nil)

				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CreateUserTxParams) (db.CreateUserTxResult, error) {
						err := arg.AfterCreate(createdUser)
						if err != nil {
							return db.CreateUserTxResult{}, err
						}

						return db.CreateUserTxResult{
							User: createdUser,
						}, nil
					})

				authenticateReq := &authPb.AuthenticateRequest{
					WalletAddress: createdUser.WalletAddress,
					UserId:        createdUser.ID.String(),
					Signature:     authenticateUserReqParams.Signature,
				}

				authService.EXPECT().
					Authenticate(gomock.Any(), gomock.Any(), authenticateReq).
					Times(1).
					Return(&authPb.AuthenticateResponse{}, db.ErrGamerTagAlreadyInUse)
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, GamerTagInUse)
				require.Empty(t, res)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newTestDeps(t)

			tc.buildStubs(deps.store, deps.authService)

			h := newTestHandler(deps)

			res, err := h.AuthenticateUser(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
