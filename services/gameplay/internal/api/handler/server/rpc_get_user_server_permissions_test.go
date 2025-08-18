package server

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
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetUserServerPermissions(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.GetUserServerPermissionsRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.GetUserServerPermissionsResponse, err error)
	}{
		{
			name: "success",
			req: &pb.GetUserServerPermissionsRequest{
				UserId:   uuid.New().String(),
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{
						IsOwner:                false,
						IsAdmin:                true,
						HasElevatedPermissions: true,
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetUserServerPermissionsResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.False(t, res.IsOwner)
				require.True(t, res.IsAdmin)
				require.True(t, res.HasElevatedPermissions)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.GetUserServerPermissionsRequest{
				UserId:   "fake-id",
				ServerId: "fake-id",
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.GetUserServerPermissionsResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedViolations := []string{"userId", "serverId"}
				handler.CheckInvalidRequestParams(t, err, expectedViolations)
			},
		},
		{
			name: "could not get server user permissions",
			req: &pb.GetUserServerPermissionsRequest{
				UserId:   uuid.New().String(),
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{}, errors.New("could not get server user permissions"))
			},
			checkResponse: func(t *testing.T, res *pb.GetUserServerPermissionsResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "no record found in db - GetServerUserPermissions",
			req: &pb.GetUserServerPermissionsRequest{
				UserId:   uuid.New().String(),
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerUserPermissions(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetServerUserPermissionsRow{}, db.RecordNotFoundError)
			},
			checkResponse: func(t *testing.T, res *pb.GetUserServerPermissionsResponse, err error) {
				require.NoError(t, err)
				require.Empty(t, res)

				require.False(t, res.IsOwner)
				require.False(t, res.IsAdmin)
				require.False(t, res.HasElevatedPermissions)
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

			tc.buildStubs(store)

			h := newTestHandler(store, cache, authService)

			res, err := h.GetUserServerPermissions(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
