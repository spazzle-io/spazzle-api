package word

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

func TestRemoveAllWords(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.RemoveAllWordsRequest
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.RemoveAllWordsResponse, err error)
	}{
		{
			name: "success",
			req: &pb.RemoveAllWordsRequest{
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
						IsOwner:                true,
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					RemoveAllServerWordsTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.RemoveAllServerWordsTxResult{
						NumWordsRemoved: int32(2),
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.RemoveAllWordsResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
				require.Equal(t, res.NumWordsRemoved, int32(2))
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.RemoveAllWordsRequest{
				ServerId: "fake-id",
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, res *pb.RemoveAllWordsResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"serverId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "could not verify user access token",
			req: &pb.RemoveAllWordsRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("could not verify user access token"))
			},
			checkResponse: func(t *testing.T, res *pb.RemoveAllWordsResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "could not get user server permissions",
			req: &pb.RemoveAllWordsRequest{
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
					Return(db.GetServerUserPermissionsRow{}, errors.New("failed to get user server permissions"))
			},
			checkResponse: func(t *testing.T, res *pb.RemoveAllWordsResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "user does not have elevated permissions on server",
			req: &pb.RemoveAllWordsRequest{
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
					Return(db.GetServerUserPermissionsRow{}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.RemoveAllWordsResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not remove all words from db",
			req: &pb.RemoveAllWordsRequest{
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
						IsOwner:                true,
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					RemoveAllServerWordsTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.RemoveAllServerWordsTxResult{}, errors.New("could not remove all words from db"))
			},
			checkResponse: func(t *testing.T, res *pb.RemoveAllWordsResponse, err error) {
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

			res, err := h.RemoveAllWords(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
