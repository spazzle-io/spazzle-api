package server_admin

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	mockservices "github.com/spazzle-io/spazzle-api/services/gameplay/internal/services/mock"
	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRemoveServerAdmin(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.RemoveServerAdminRequest
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.RemoveServerAdminResponse, err error)
	}{
		{
			name: "success",
			req: &pb.RemoveServerAdminRequest{
				UserId:   uuid.New().String(),
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
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
					RemoveServerAdminTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.RemoveServerAdminResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
				require.NotEmpty(t, res.GetUserId())
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.RemoveServerAdminRequest{
				UserId:   "fake-id",
				ServerId: "fake-id",
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, res *pb.RemoveServerAdminResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"serverId", "userId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not verify access token",
			req: &pb.RemoveServerAdminRequest{
				UserId:   uuid.New().String(),
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("could not verify access token"))
			},
			checkResponse: func(t *testing.T, res *pb.RemoveServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "invalid user id",
			req: &pb.RemoveServerAdminRequest{
				UserId:   uuid.New().String(),
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: "fake-id",
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.RemoveServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not get user server permissions",
			req: &pb.RemoveServerAdminRequest{
				UserId:   uuid.New().String(),
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{}, errors.New("could not get user server permissions"))

				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.RemoveServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "user does not have permissions to remove server admin",
			req: &pb.RemoveServerAdminRequest{
				UserId:   uuid.New().String(),
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						IsOwner: false,
					}, nil)

				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: uuid.New().String(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.RemoveServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "server not found",
			req: &pb.RemoveServerAdminRequest{
				UserId:   uuid.New().String(),
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
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
					RemoveServerAdminTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ErrServerNotfound)
			},
			checkResponse: func(t *testing.T, res *pb.RemoveServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.ServerNotFoundError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not remove server admin in db",
			req: &pb.RemoveServerAdminRequest{
				UserId:   uuid.New().String(),
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
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
					RemoveServerAdminTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("could not remove server admin"))
			},
			checkResponse: func(t *testing.T, res *pb.RemoveServerAdminResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newTestDeps(t)

			tc.buildStubs(deps.store, deps.authService)

			h := newTestHandler(deps)

			res, err := h.RemoveServerAdmin(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
