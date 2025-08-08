package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/users/users/v1"
	mockdb "github.com/spazzle-io/spazzle-api/services/users/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/users/internal/db/sqlc"
	mockservices "github.com/spazzle-io/spazzle-api/services/users/internal/services/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func generateCreateUserReqParams(t *testing.T) *pb.CreateUserRequest {
	wallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, wallet)

	gamerTag := gofakeit.Gamertag()
	require.NotNil(t, gamerTag)

	return &pb.CreateUserRequest{
		WalletAddress: wallet.Address,
		GamerTag:      gamerTag,
		Signature:     "some-valid-signature",
	}
}

func TestCreateUser(t *testing.T) {
	createUserReqParams := generateCreateUserReqParams(t)
	require.NotEmpty(t, createUserReqParams)

	testCases := []struct {
		name          string
		req           *pb.CreateUserRequest
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.CreateUserResponse, err error)
	}{
		{
			name: "success",
			req:  createUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				createdUser := db.User{
					ID:            uuid.New(),
					WalletAddress: createUserReqParams.GetWalletAddress(),
					GamerTag: pgtype.Text{
						String: createUserReqParams.GetGamerTag(),
						Valid:  true,
					},
					CreatedAt: time.Now().UTC(),
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

				authenticateReq := &authPb.AuthenticateRequest{
					WalletAddress: createUserReqParams.GetWalletAddress(),
					UserId:        createdUser.ID.String(),
					Signature:     createUserReqParams.GetSignature(),
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
			checkResponse: func(t *testing.T, res *pb.CreateUserResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.NotEmpty(t, res.GetUser())
				require.NotEmpty(t, res.GetSession())

				require.Equal(t, createUserReqParams.GetWalletAddress(), res.GetUser().GetWalletAddress())
				require.Equal(t, createUserReqParams.GetGamerTag(), res.GetUser().GetGamerTag())
			},
		},
		{
			name: "invalid request arguments",
			req: &pb.CreateUserRequest{
				WalletAddress: "invalid-address",
				Signature:     "",
				GamerTag:      "",
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, res *pb.CreateUserResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"walletAddress", "signature"}
				checkInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "user already exists",
			req:  createUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CreateUserTxParams) (db.CreateUserTxResult, error) {
						return db.CreateUserTxResult{}, db.ErrUserAlreadyExists
					})
			},
			checkResponse: func(t *testing.T, res *pb.CreateUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UserAlreadyExists)
				require.Empty(t, res)
			},
		},
		{
			name: "gamer tag already in use",
			req:  createUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CreateUserTxParams) (db.CreateUserTxResult, error) {
						return db.CreateUserTxResult{}, db.ErrGamerTagAlreadyInUse
					})
			},
			checkResponse: func(t *testing.T, res *pb.CreateUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, GamerTagInUse)
				require.Empty(t, res)
			},
		},
		{
			name: "could not create user tx",
			req:  createUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CreateUserTxParams) (db.CreateUserTxResult, error) {
						return db.CreateUserTxResult{}, errors.New("could not create user tx")
					})
			},
			checkResponse: func(t *testing.T, res *pb.CreateUserResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not authenticate user",
			req:  createUserReqParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				createdUser := db.User{
					ID:            uuid.New(),
					WalletAddress: createUserReqParams.GetWalletAddress(),
					GamerTag: pgtype.Text{
						String: createUserReqParams.GetGamerTag(),
						Valid:  true,
					},
					CreatedAt: time.Now().UTC(),
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

				authenticateReq := &authPb.AuthenticateRequest{
					WalletAddress: createUserReqParams.GetWalletAddress(),
					UserId:        createdUser.ID.String(),
					Signature:     createUserReqParams.GetSignature(),
				}

				authService.EXPECT().
					Authenticate(gomock.Any(), "test", authenticateReq).
					Times(1).
					Return(&authPb.AuthenticateResponse{}, errors.New("could not authenticate user"))
			},
			checkResponse: func(t *testing.T, res *pb.CreateUserResponse, err error) {
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

			res, err := handler.CreateUser(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
