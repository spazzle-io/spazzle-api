package word

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/wrappers"
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

func TestListWords(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.ListWordsRequest
		buildStubs    func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.ListWordsResponse, err error)
	}{
		{
			name: "success",
			req: &pb.ListWordsRequest{
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
						IsOwner:                true,
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:             uuid.New(),
						NumCustomWords: 3,
					}, nil)

				store.EXPECT().
					ListWords(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListWordsRow{
						{
							ID:       uuid.New(),
							ServerID: uuid.New(),
							Word:     "bike",
							AddedAt:  time.Now().UTC(),
						},
						{
							ID:       uuid.New(),
							ServerID: uuid.New(),
							Word:     "sun",
							AddedAt:  time.Now().UTC(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListWordsResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.NotEmpty(t, res.Words)
				require.NotEmpty(t, res.Cursor)
				require.Equal(t, int64(3), res.TotalCount)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.ListWordsRequest{
				ServerId: "fake-id",
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
			},
			checkResponse: func(t *testing.T, res *pb.ListWordsResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"serverId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "invalid after id",
			req: &pb.ListWordsRequest{
				ServerId: uuid.New().String(),
				AfterId: &wrappers.StringValue{
					Value: "fake-id",
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
						IsOwner:                true,
						HasElevatedPermissions: true,
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListWordsResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InvalidAfterIdError)
				require.Empty(t, res)
			},
		},
		{
			name: "page size is zero",
			req: &pb.ListWordsRequest{
				ServerId: uuid.New().String(),
				PageSize: &wrappers.Int32Value{
					Value: 0,
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
						IsOwner:                true,
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:             uuid.New(),
						NumCustomWords: 3,
					}, nil)

				store.EXPECT().
					ListWords(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListWordsRow{
						{
							ID:       uuid.New(),
							ServerID: uuid.New(),
							Word:     "bike",
							AddedAt:  time.Now().UTC(),
						},
						{
							ID:       uuid.New(),
							ServerID: uuid.New(),
							Word:     "sun",
							AddedAt:  time.Now().UTC(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListWordsResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
				require.Equal(t, handler.DefaultPageSize, res.Cursor.PageSize)
			},
		},
		{
			name: "page size is greater than allowed maximum",
			req: &pb.ListWordsRequest{
				ServerId: uuid.New().String(),
				PageSize: &wrappers.Int32Value{
					Value: handler.MaxPageSize + 1,
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
						IsOwner:                true,
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:             uuid.New(),
						NumCustomWords: 3,
					}, nil)

				store.EXPECT().
					ListWords(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListWordsRow{
						{
							ID:       uuid.New(),
							ServerID: uuid.New(),
							Word:     "bike",
							AddedAt:  time.Now().UTC(),
						},
						{
							ID:       uuid.New(),
							ServerID: uuid.New(),
							Word:     "sun",
							AddedAt:  time.Now().UTC(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListWordsResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
				require.Equal(t, handler.DefaultPageSize, res.Cursor.PageSize)
			},
		},
		{
			name: "could not verify user access token",
			req: &pb.ListWordsRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("could not verify user access token"))
			},
			checkResponse: func(t *testing.T, res *pb.ListWordsResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "could not get user server permissions",
			req: &pb.ListWordsRequest{
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
					Return(db.GetServerUserPermissionsRow{}, errors.New("failed to get user server permissions"))
			},
			checkResponse: func(t *testing.T, res *pb.ListWordsResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "user does not have elevated permissions on server",
			req: &pb.ListWordsRequest{
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
					Return(db.GetServerUserPermissionsRow{}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListWordsResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not get server",
			req: &pb.ListWordsRequest{
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
						IsOwner:                true,
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, errors.New("could not get server"))
			},
			checkResponse: func(t *testing.T, res *pb.ListWordsResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not list words",
			req: &pb.ListWordsRequest{
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
						IsOwner:                true,
						HasElevatedPermissions: true,
					}, nil)

				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:             uuid.New(),
						NumCustomWords: 3,
					}, nil)

				store.EXPECT().
					ListWords(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListWordsRow{}, errors.New("could not list words"))
			},
			checkResponse: func(t *testing.T, res *pb.ListWordsResponse, err error) {
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

			res, err := h.ListWords(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
