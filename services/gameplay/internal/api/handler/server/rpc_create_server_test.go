package server

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	mockservices "github.com/spazzle-io/spazzle-api/services/gameplay/internal/services/mock"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func generateCreateServerReqParams(t *testing.T) *pb.CreateServerRequest {
	randStr, err := commonUtil.GenerateRandomAlphanumericString(4)
	require.NoError(t, err)
	require.NotEmpty(t, randStr)

	serverWallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, serverWallet)

	return &pb.CreateServerRequest{
		Name:              fmt.Sprintf("%s_%s", gofakeit.PetName(), randStr),
		ServerAddress:     serverWallet.Address,
		IsPubliclyVisible: true,
		StakePerGame:      "1200000000000000000",
		NumRoundsPerGame:  3,
		RoundDurationSecs: 60,
		NumDrawingOptions: 4,
	}
}

func TestCreateServer(t *testing.T) {
	createServerParams := generateCreateServerReqParams(t)
	require.NotEmpty(t, createServerParams)

	testCases := []struct {
		name          string
		req           *pb.CreateServerRequest
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.CreateServerResponse, err error)
	}{
		{
			name: "success",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)

				store.EXPECT().
					CreateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID: uuid.New(),
						StakePerGame: pgtype.Numeric{
							Int:   big.NewInt(12),
							Valid: true,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.CreateServerResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.NotEmpty(t, res.GetServer())
			},
		},
		{
			name:       "invalid request parameters",
			req:        &pb.CreateServerRequest{},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, res *pb.CreateServerResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res.GetServer())

				expectedFieldViolations := []string{"name", "serverAddress", "serverAddress", "stakePerGame", "numRoundsPerGame", "roundDurationSecs", "numDrawingOptions"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not verify access token",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("could not verify access token"))
			},
			checkResponse: func(t *testing.T, res *pb.CreateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "invalid user id",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: "fake-id",
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.CreateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "invalid stake per game in request",
			req: &pb.CreateServerRequest{
				Name:              createServerParams.Name,
				ServerAddress:     createServerParams.ServerAddress,
				IsPubliclyVisible: false,
				StakePerGame:      "abc",
				NumRoundsPerGame:  createServerParams.NumRoundsPerGame,
				RoundDurationSecs: createServerParams.RoundDurationSecs,
				NumDrawingOptions: createServerParams.NumDrawingOptions,
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.CreateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InvalidStakePerGameError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not create server in db - server name already exists",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)

				store.EXPECT().
					CreateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, &pgconn.PgError{
						Code:           db.UniqueViolationCode,
						ConstraintName: "servers_name_unique_unarchived_idx",
					})
			},
			checkResponse: func(t *testing.T, res *pb.CreateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.ServerNameInUseError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not create server in db - unknown error",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					CreateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, errors.New("unknown error"))

				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.CreateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "invalid stake per game in db",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)

				store.EXPECT().
					CreateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID: uuid.New(),
						StakePerGame: pgtype.Numeric{
							Valid: false,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.CreateServerResponse, err error) {
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

			tc.buildStubs(store, authService)

			serverHandler := newTestHandler(store, cache, authService)

			res, err := serverHandler.CreateServer(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
