package server

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	mocktreasury "github.com/spazzle-io/spazzle-api/services/gameplay/internal/treasury/mock"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

	return &pb.CreateServerRequest{
		Name:              fmt.Sprintf("%s_%s", gofakeit.PetName(), randStr),
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
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient)
		checkResponse func(t *testing.T, res *pb.CreateServerResponse, err error)
	}{
		{
			name: "success",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId:        uuid.New().String(),
							WalletAddress: common.HexToAddress("0x123").Hex(),
						},
					}, nil)

				treasuryClient.EXPECT().
					PredictAddress(gomock.Any(), gomock.Eq(common.HexToAddress("0x123"))).
					Times(1).
					Return(common.HexToAddress("0x456"), nil)

				store.EXPECT().
					CreateServerTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateServerTxResult{
						Server: db.Server{
							ID: uuid.New(),
							StakePerGame: pgtype.Numeric{
								Int:   big.NewInt(12),
								Valid: true,
							},
							TotalVolume: pgtype.Numeric{
								Valid: true,
							},
							ServerAddress: common.HexToAddress("0x456").Hex(),
						},
						Treasury: db.ServerTreasury{
							Address:  common.HexToAddress("0x456").Hex(),
							ServerID: uuid.New(),
							Owner:    common.HexToAddress("0x123").Hex(),
							Status:   db.TreasuryStatusDeployed,
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
			name: "invalid request parameters",
			req:  &pb.CreateServerRequest{},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
			},
			checkResponse: func(t *testing.T, res *pb.CreateServerResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res.GetServer())

				expectedFieldViolations := []string{"name", "stakePerGame", "numRoundsPerGame", "roundDurationSecs", "numDrawingOptions"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not verify access token",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
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
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: "invalid-user-id",
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
			name: "invalid stake per game",
			req: &pb.CreateServerRequest{
				Name:              createServerParams.Name,
				StakePerGame:      "abc",
				NumRoundsPerGame:  createServerParams.NumRoundsPerGame,
				RoundDurationSecs: createServerParams.RoundDurationSecs,
				NumDrawingOptions: createServerParams.NumDrawingOptions,
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
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
			name: "negative stake per game",
			req: &pb.CreateServerRequest{
				Name:              createServerParams.Name,
				StakePerGame:      "-1000",
				NumRoundsPerGame:  createServerParams.NumRoundsPerGame,
				RoundDurationSecs: createServerParams.RoundDurationSecs,
				NumDrawingOptions: createServerParams.NumDrawingOptions,
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
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
			name: "zero stake per game",
			req: &pb.CreateServerRequest{
				Name:              createServerParams.Name,
				StakePerGame:      "0",
				NumRoundsPerGame:  createServerParams.NumRoundsPerGame,
				RoundDurationSecs: createServerParams.RoundDurationSecs,
				NumDrawingOptions: createServerParams.NumDrawingOptions,
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId:        uuid.New().String(),
							WalletAddress: common.HexToAddress("0x123").Hex(),
						},
					}, nil)

				treasuryClient.EXPECT().
					PredictAddress(gomock.Any(), gomock.Eq(common.HexToAddress("0x123"))).
					Times(1).
					Return(common.HexToAddress("0x456"), nil)

				store.EXPECT().
					CreateServerTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateServerTxResult{
						Server: db.Server{
							ID: uuid.New(),
							StakePerGame: pgtype.Numeric{
								Int:   big.NewInt(12),
								Valid: true,
							},
							TotalVolume: pgtype.Numeric{
								Valid: true,
							},
							ServerAddress: common.HexToAddress("0x456").Hex(),
						},
						Treasury: db.ServerTreasury{
							Address:  common.HexToAddress("0x456").Hex(),
							ServerID: uuid.New(),
							Owner:    common.HexToAddress("0x123").Hex(),
							Status:   db.TreasuryStatusDeployed,
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
			name: "invalid owner address",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId:        uuid.New().String(),
							WalletAddress: "invalid-owner-address",
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
			name: "missing owner address",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
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
			name: "failed to predict treasury address",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId:        uuid.New().String(),
							WalletAddress: common.HexToAddress("0x123").Hex(),
						},
					}, nil)

				treasuryClient.EXPECT().
					PredictAddress(gomock.Any(), gomock.Eq(common.HexToAddress("0x123"))).
					Times(1).
					Return(common.Address{}, errors.New("failed to predict treasury address"))
			},
			checkResponse: func(t *testing.T, res *pb.CreateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "create server db tx failed",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId:        uuid.New().String(),
							WalletAddress: common.HexToAddress("0x123").Hex(),
						},
					}, nil)

				treasuryClient.EXPECT().
					PredictAddress(gomock.Any(), gomock.Eq(common.HexToAddress("0x123"))).
					Times(1).
					Return(common.HexToAddress("0x456"), nil)

				store.EXPECT().
					CreateServerTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateServerTxResult{}, errors.New("create server tx failed"))
			},
			checkResponse: func(t *testing.T, res *pb.CreateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not map db server to pb",
			req:  createServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService, treasuryClient *mocktreasury.MockClient) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId:        uuid.New().String(),
							WalletAddress: common.HexToAddress("0x123").Hex(),
						},
					}, nil)

				treasuryClient.EXPECT().
					PredictAddress(gomock.Any(), gomock.Eq(common.HexToAddress("0x123"))).
					Times(1).
					Return(common.HexToAddress("0x456"), nil)

				store.EXPECT().
					CreateServerTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateServerTxResult{
						Server: db.Server{
							ID: uuid.New(),
							StakePerGame: pgtype.Numeric{
								Int:   big.NewInt(12),
								Valid: true,
							},
							TotalVolume: pgtype.Numeric{
								Valid: false,
							},
							ServerAddress: common.HexToAddress("0x456").Hex(),
						},
						Treasury: db.ServerTreasury{
							Address:  common.HexToAddress("0x456").Hex(),
							ServerID: uuid.New(),
							Owner:    common.HexToAddress("0x123").Hex(),
							Status:   db.TreasuryStatusDeployed,
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
			deps := newTestDeps(t)

			tc.buildStubs(deps.store, deps.authService, deps.treasuryClient)

			h := newTestHandler(deps)

			res, err := h.CreateServer(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
