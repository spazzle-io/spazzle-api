package handler

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	mockdb "github.com/spazzle-io/spazzle-api/services/auth/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/siwe"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func generateTestAuthenticateReqParams(t *testing.T) (*siwe.Payload, *pb.AuthenticateRequest) {
	wallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, wallet)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache := mockcache.NewMockCache(ctrl)

	cache.EXPECT().
		Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		Return(nil)

	config := getTestConfig()
	siwePayload, err := siwe.GenerateSIWEPayload(
		context.Background(),
		config,
		cache,
		"localhost",
		"http://localhost:3000/login",
		2021,
		wallet.Address,
	)
	require.NoError(t, err)
	require.NotEmpty(t, siwePayload)

	signature, err := commonUtil.SignMessageEthereum(wallet.PrivateKey, siwePayload.Message)
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	return siwePayload, &pb.AuthenticateRequest{
		WalletAddress: wallet.Address,
		UserId:        uuid.New().String(),
		Signature:     signature,
	}
}

func TestAuthenticate(t *testing.T) {
	siwePayload, authenticateReqParams := generateTestAuthenticateReqParams(t)
	require.NotEmpty(t, siwePayload)
	require.NotEmpty(t, authenticateReqParams)

	userId, err := uuid.Parse(authenticateReqParams.UserId)
	require.NoError(t, err)
	require.NotEmpty(t, userId)

	testCases := []struct {
		name          string
		req           *pb.AuthenticateRequest
		inputCtx      context.Context
		buildStubs    func(store *mockdb.MockStore, cache *mockcache.MockCache)
		checkResponse func(t *testing.T, res *pb.AuthenticateResponse, err error)
	}{
		{
			name:     "success - new credential",
			req:      authenticateReqParams,
			inputCtx: context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "users"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(siwePayload.Message, nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					GetCredentialByWalletAddress(gomock.Any(), siwePayload.WalletAddress).
					Times(1).
					Return(db.Credential{}, db.RecordNotFoundError)

				store.EXPECT().
					CreateCredentialTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CreateCredentialTxParams) (db.CreateCredentialTxResult, error) {
						credential := db.Credential{
							ID:            uuid.New(),
							UserID:        userId,
							WalletAddress: siwePayload.WalletAddress,
							Role:          db.Role(token.User),
							CreatedAt:     time.Now().UTC(),
						}

						err := arg.AfterCreate(credential)
						if err != nil {
							return db.CreateCredentialTxResult{}, err
						}

						return db.CreateCredentialTxResult{
							Credential: credential,
						}, nil
					})

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{
						ID: uuid.New(),
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.Equal(t, authenticateReqParams.WalletAddress, res.GetCredential().GetWalletAddress())
				require.Equal(t, authenticateReqParams.UserId, res.GetCredential().GetUserId())
				require.Equal(t, "bearer", res.GetSession().GetTokenType())
			},
		},
		{
			name:     "success - existing credential",
			req:      authenticateReqParams,
			inputCtx: context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "users"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(siwePayload.Message, nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					GetCredentialByWalletAddress(gomock.Any(), siwePayload.WalletAddress).
					Times(1).
					Return(db.Credential{
						ID:            uuid.New(),
						UserID:        userId,
						WalletAddress: siwePayload.WalletAddress,
						Role:          db.Role(token.User),
						CreatedAt:     time.Now().UTC(),
					}, nil)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{
						ID: uuid.New(),
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.Equal(t, authenticateReqParams.WalletAddress, res.GetCredential().GetWalletAddress())
				require.Equal(t, authenticateReqParams.UserId, res.GetCredential().GetUserId())
				require.Equal(t, "bearer", res.GetSession().GetTokenType())
			},
		},
		{
			name: "invalid request arguments",
			req: &pb.AuthenticateRequest{
				WalletAddress: "invalid-address",
				UserId:        "123",
				Signature:     "",
			},
			inputCtx:   context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "users"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"walletAddress", "userId", "signature"}
				checkInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name:       "request not made by an authenticated service",
			req:        authenticateReqParams,
			inputCtx:   context.Background(),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name:       "request not made by an allowed service",
			req:        authenticateReqParams,
			inputCtx:   context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "some_other_service"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name:     "could not get cached SIWE message",
			req:      authenticateReqParams,
			inputCtx: context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "users"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, errors.New("some cache error"))
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "invalid signature",
			req: &pb.AuthenticateRequest{
				WalletAddress: siwePayload.WalletAddress,
				UserId:        authenticateReqParams.UserId,
				Signature:     fmt.Sprintf("%sz", authenticateReqParams.GetSignature()[:len(authenticateReqParams.GetSignature())-1]),
			},
			inputCtx: context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "users"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(siwePayload.Message, nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, SignatureVerificationError)
				require.Empty(t, res)
			},
		},
		{
			name:     "could not get credential by wallet address",
			req:      authenticateReqParams,
			inputCtx: context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "users"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(siwePayload.Message, nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					GetCredentialByWalletAddress(gomock.Any(), siwePayload.WalletAddress).
					Times(1).
					Return(db.Credential{}, errors.New("some db error"))
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name:     "could not create credential",
			req:      authenticateReqParams,
			inputCtx: context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "users"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(siwePayload.Message, nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					GetCredentialByWalletAddress(gomock.Any(), siwePayload.WalletAddress).
					Times(1).
					Return(db.Credential{}, nil)

				store.EXPECT().
					CreateCredentialTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCredentialTxResult{}, errors.New("some db error"))
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name:     "credential already exists",
			req:      authenticateReqParams,
			inputCtx: context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "users"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(siwePayload.Message, nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					GetCredentialByWalletAddress(gomock.Any(), siwePayload.WalletAddress).
					Times(1).
					Return(db.Credential{}, nil)

				store.EXPECT().
					CreateCredentialTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateCredentialTxResult{}, db.ErrCredentialAlreadyExists)
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, credentialAlreadyExistsError)
				require.Empty(t, res)
			},
		},
		{
			name:     "could not create session for new credential",
			req:      authenticateReqParams,
			inputCtx: context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "users"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(siwePayload.Message, nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					GetCredentialByWalletAddress(gomock.Any(), siwePayload.WalletAddress).
					Times(1).
					Return(db.Credential{}, nil)

				store.EXPECT().
					CreateCredentialTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(_ context.Context, arg db.CreateCredentialTxParams) (db.CreateCredentialTxResult, error) {
						credential := db.Credential{
							ID:            uuid.New(),
							UserID:        userId,
							WalletAddress: siwePayload.WalletAddress,
							Role:          db.Role(token.User),
							CreatedAt:     time.Now().UTC(),
						}

						err := arg.AfterCreate(credential)
						if err != nil {
							return db.CreateCredentialTxResult{}, err
						}

						return db.CreateCredentialTxResult{
							Credential: credential,
						}, nil
					})

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{}, errors.New("some db error"))
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name:     "provided user id does not match credential",
			req:      authenticateReqParams,
			inputCtx: context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "users"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(siwePayload.Message, nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					GetCredentialByWalletAddress(gomock.Any(), siwePayload.WalletAddress).
					Times(1).
					Return(db.Credential{
						ID:            uuid.New(),
						UserID:        uuid.New(),
						WalletAddress: siwePayload.WalletAddress,
						Role:          db.Role(token.User),
						CreatedAt:     time.Now().UTC(),
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name:     "could not create session for existing credential",
			req:      authenticateReqParams,
			inputCtx: context.WithValue(context.Background(), commonMiddleware.AuthenticatedService, "users"),
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any()).
					Times(1).
					Return(siwePayload.Message, nil)

				cache.EXPECT().
					Del(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					GetCredentialByWalletAddress(gomock.Any(), siwePayload.WalletAddress).
					Times(1).
					Return(db.Credential{
						ID:            uuid.New(),
						UserID:        userId,
						WalletAddress: siwePayload.WalletAddress,
						Role:          db.Role(token.User),
						CreatedAt:     time.Now().UTC(),
					}, nil)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{}, errors.New("some db error"))
			},
			checkResponse: func(t *testing.T, res *pb.AuthenticateResponse, err error) {
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

			tc.buildStubs(store, cache)

			handler := newTestHandler(t, store, cache)

			res, err := handler.Authenticate(tc.inputCtx, tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
