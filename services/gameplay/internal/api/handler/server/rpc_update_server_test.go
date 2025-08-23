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
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func generateUpdateServerReqParams(t *testing.T) *pb.UpdateServerRequest {
	randStr, err := commonUtil.GenerateRandomAlphanumericString(4)
	require.NoError(t, err)
	require.NotEmpty(t, randStr)

	return &pb.UpdateServerRequest{
		ServerId:          uuid.New().String(),
		Name:              &wrapperspb.StringValue{Value: fmt.Sprintf("%s_%s", gofakeit.PetName(), randStr)},
		IsPubliclyVisible: &wrapperspb.BoolValue{Value: true},
		StakePerGame:      &wrapperspb.StringValue{Value: "150000"},
		NumRoundsPerGame:  &wrapperspb.Int32Value{Value: 8},
		RoundDurationSecs: &wrapperspb.Int32Value{Value: 60},
		NumDrawingOptions: &wrapperspb.Int32Value{Value: 5},
	}
}

func TestUpdateServer(t *testing.T) {
	updateServerParams := generateUpdateServerReqParams(t)
	require.NotEmpty(t, updateServerParams)

	testCases := []struct {
		name          string
		req           *pb.UpdateServerRequest
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.UpdateServerResponse, err error)
	}{
		{
			name: "success - all fields populated",
			req:  updateServerParams,
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
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					UpdateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID: uuid.New(),
						StakePerGame: pgtype.Numeric{
							Int:   big.NewInt(12),
							Valid: true,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateServerResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
				require.NotEmpty(t, res.GetServer())
			},
		},
		{
			name: "success - no fields populated",
			req: &pb.UpdateServerRequest{
				ServerId: uuid.New().String(),
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

				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					UpdateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID: uuid.New(),
						StakePerGame: pgtype.Numeric{
							Int:   big.NewInt(12),
							Valid: true,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateServerResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
				require.NotEmpty(t, res.GetServer())
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.UpdateServerRequest{
				ServerId: "fake-server-id",
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, res *pb.UpdateServerResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res.GetServer())

				expectedFieldViolations := []string{"serverId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not verify access token",
			req:  updateServerParams,
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("could not verify access token"))
			},
			checkResponse: func(t *testing.T, res *pb.UpdateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "invalid user id",
			req:  updateServerParams,
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
			checkResponse: func(t *testing.T, res *pb.UpdateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not get server user permissions",
			req:  updateServerParams,
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
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{}, errors.New("could not get server user permissions"))
			},
			checkResponse: func(t *testing.T, res *pb.UpdateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "user does not have elevated permissions on the server",
			req:  updateServerParams,
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
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: false,
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "invalid stake per game in request",
			req: &pb.UpdateServerRequest{
				ServerId: uuid.New().String(),
				StakePerGame: &wrapperspb.StringValue{
					Value: "abc",
				},
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

				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: true,
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InvalidStakePerGameError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not update server",
			req:  updateServerParams,
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
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					UpdateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, errors.New("could not update server"))
			},
			checkResponse: func(t *testing.T, res *pb.UpdateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "server name already in use",
			req:  updateServerParams,
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
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					UpdateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, &pgconn.PgError{
						Code:           db.UniqueViolationCode,
						ConstraintName: "servers_name_unique_unarchived_idx",
					})
			},
			checkResponse: func(t *testing.T, res *pb.UpdateServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.ServerNameInUseError)
				require.Empty(t, res)
			},
		},
		{
			name: "invalid stake per game from db",
			req:  updateServerParams,
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
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					UpdateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID: uuid.New(),
						StakePerGame: pgtype.Numeric{
							Valid: false,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateServerResponse, err error) {
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

			res, err := serverHandler.UpdateServer(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
