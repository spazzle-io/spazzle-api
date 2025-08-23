package server

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	mockservices "github.com/spazzle-io/spazzle-api/services/gameplay/internal/services/mock"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestArchiveServer(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.ArchiveServerRequest
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.ArchiveServerResponse, err error)
	}{
		{
			name: "success",
			req: &pb.ArchiveServerRequest{
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
					UpdateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID: uuid.New(),
						StakePerGame: pgtype.Numeric{
							Int:   big.NewInt(10),
							Valid: true,
						},
					}, nil)

				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						IsOwner: true,
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ArchiveServerResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
				require.NotEmpty(t, res.GetServer())
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.ArchiveServerRequest{
				ServerId: "fake-id",
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, res *pb.ArchiveServerResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"serverId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not verify access token",
			req: &pb.ArchiveServerRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("could not verify access token"))
			},
			checkResponse: func(t *testing.T, res *pb.ArchiveServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "invalid user id",
			req: &pb.ArchiveServerRequest{
				ServerId: uuid.New().String(),
			},
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
			checkResponse: func(t *testing.T, res *pb.ArchiveServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not get server user permissions",
			req: &pb.ArchiveServerRequest{
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
					Return(db.GetServerUserPermissionsRow{}, errors.New("could not get server user permissions"))
			},
			checkResponse: func(t *testing.T, res *pb.ArchiveServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "user does not have permission to archive server",
			req: &pb.ArchiveServerRequest{
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
						IsOwner: false,
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ArchiveServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not update server",
			req: &pb.ArchiveServerRequest{
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
						IsOwner: true,
					}, nil)

				store.EXPECT().
					UpdateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, errors.New("could not update server"))
			},
			checkResponse: func(t *testing.T, res *pb.ArchiveServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "server not found",
			req: &pb.ArchiveServerRequest{
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
						IsOwner: true,
					}, nil)

				store.EXPECT().
					UpdateServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, db.RecordNotFoundError)
			},
			checkResponse: func(t *testing.T, res *pb.ArchiveServerResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.ServerNotFoundError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not map db server to pb",
			req: &pb.ArchiveServerRequest{
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
						IsOwner: true,
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
			checkResponse: func(t *testing.T, res *pb.ArchiveServerResponse, err error) {
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

			res, err := serverHandler.ArchiveServer(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
