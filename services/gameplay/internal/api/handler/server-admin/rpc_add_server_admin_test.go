package server_admin

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
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

func TestAddServerAdmin(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.AddServerAdminRequest
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.AddServerAdminResponse, err error)
	}{
		{
			name: "success",
			req: &pb.AddServerAdminRequest{
				ServerId: uuid.New().String(),
				UserId:   uuid.New().String(),
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
					AddServerAdminTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.AddServerAdminTxResult{
						ServerAdmin: db.ServerAdmin{
							ServerID: uuid.New(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.AddServerAdminResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
				require.NotEmpty(t, res.GetAdmin())
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.AddServerAdminRequest{
				ServerId: "fake-id",
				UserId:   "fake-id",
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, res *pb.AddServerAdminResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"serverId", "userId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not verify access token",
			req: &pb.AddServerAdminRequest{
				ServerId: uuid.New().String(),
				UserId:   uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("could not verify access token"))
			},
			checkResponse: func(t *testing.T, res *pb.AddServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "invalid user id",
			req: &pb.AddServerAdminRequest{
				ServerId: uuid.New().String(),
				UserId:   uuid.New().String(),
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
			checkResponse: func(t *testing.T, res *pb.AddServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not get user server permissions",
			req: &pb.AddServerAdminRequest{
				ServerId: uuid.New().String(),
				UserId:   uuid.New().String(),
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
					Return(db.GetServerUserPermissionsRow{}, errors.New("could not get user server permissions"))
			},
			checkResponse: func(t *testing.T, res *pb.AddServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "user does not have permissions to add server admin",
			req: &pb.AddServerAdminRequest{
				ServerId: uuid.New().String(),
				UserId:   uuid.New().String(),
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
			checkResponse: func(t *testing.T, res *pb.AddServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "server not found",
			req: &pb.AddServerAdminRequest{
				ServerId: uuid.New().String(),
				UserId:   uuid.New().String(),
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
					AddServerAdminTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.AddServerAdminTxResult{}, db.ErrServerNotfound)
			},
			checkResponse: func(t *testing.T, res *pb.AddServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.ServerNotFoundError)
				require.Empty(t, res)
			},
		},
		{
			name: "user is already an admin",
			req: &pb.AddServerAdminRequest{
				ServerId: uuid.New().String(),
				UserId:   uuid.New().String(),
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
					AddServerAdminTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.AddServerAdminTxResult{}, db.ErrUserAlreadyAdmin)
			},
			checkResponse: func(t *testing.T, res *pb.AddServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, db.ErrUserAlreadyAdmin.Error())
				require.Empty(t, res)
			},
		},
		{
			name: "could not add server admin in db",
			req: &pb.AddServerAdminRequest{
				ServerId: uuid.New().String(),
				UserId:   uuid.New().String(),
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
					AddServerAdminTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.AddServerAdminTxResult{}, errors.New("could not add server admin"))
			},
			checkResponse: func(t *testing.T, res *pb.AddServerAdminResponse, err error) {
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

			res, err := serverHandler.AddServerAdmin(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
