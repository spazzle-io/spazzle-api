package middleware

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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestResolveServerUserContext(t *testing.T) {
	testCases := []struct {
		name          string
		serverId      string
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, serverUserContext ServerUserContext, err error)
	}{
		{
			name:     "success",
			serverId: uuid.New().String(),
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
						IsOwner:                true,
						HasElevatedPermissions: true,
					}, nil)
			},
			checkResponse: func(t *testing.T, serverUserContext ServerUserContext, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, serverUserContext.ServerId)
				require.NotEmpty(t, serverUserContext.UserId)
				require.NotEmpty(t, serverUserContext.AccessTokenPayload.UserId)
				require.False(t, serverUserContext.UserServerPermissions.IsAdmin)
				require.True(t, serverUserContext.UserServerPermissions.IsOwner)
				require.True(t, serverUserContext.UserServerPermissions.HasElevatedPermissions)
			},
		},
		{
			name:       "invalid server id",
			serverId:   "invalid-server-id",
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, serverUserContext ServerUserContext, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InvalidServerIdError)
				require.Empty(t, serverUserContext)
			},
		},
		{
			name:     "could not verify access token",
			serverId: uuid.New().String(),
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("could not verify access token"))
			},
			checkResponse: func(t *testing.T, serverUserContext ServerUserContext, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
			},
		},
		{
			name:     "invalid user id",
			serverId: uuid.New().String(),
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: "invalid-user-id",
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, serverUserContext ServerUserContext, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
			},
		},
		{
			name:     "could not get server user permissions",
			serverId: uuid.New().String(),
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
			checkResponse: func(t *testing.T, serverUserContext ServerUserContext, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			authService := mockservices.NewMockAuthGrpcService(ctrl)

			tc.buildStubs(store, authService)

			serverUserCtx, err := ResolveServerUserContext(context.Background(), tc.serverId, "test", store, authService)
			tc.checkResponse(t, serverUserCtx, err)
		})
	}
}
