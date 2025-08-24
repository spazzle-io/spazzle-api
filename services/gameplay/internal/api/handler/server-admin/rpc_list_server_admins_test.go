package server_admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/wrappers"
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

func TestListServerAdmins(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.ListServerAdminsRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.ListServerAdminsResponse, err error)
	}{
		{
			name: "success",
			req: &pb.ListServerAdminsRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:        uuid.New(),
						NumAdmins: 1,
					}, nil)

				store.EXPECT().
					ListServerAdmins(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ServerAdmin{
						{
							ServerID: uuid.New(),
							UserID:   uuid.New(),
							AddedAt:  time.Now().UTC(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListServerAdminsResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.NotEmpty(t, res.Admins)
				require.NotEmpty(t, res.Cursor)
				require.Equal(t, int64(1), res.TotalCount)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.ListServerAdminsRequest{
				ServerId: "fake-id",
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.ListServerAdminsResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"serverId"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "invalid after user id",
			req: &pb.ListServerAdminsRequest{
				ServerId: uuid.New().String(),
				AfterUserId: &wrappers.StringValue{
					Value: "fake-server-id",
				},
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.ListServerAdminsResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InvalidAfterIdError)
				require.Empty(t, res)
			},
		},
		{
			name: "page size is zero",
			req: &pb.ListServerAdminsRequest{
				ServerId: uuid.New().String(),
				PageSize: &wrappers.Int32Value{
					Value: 0,
				},
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:        uuid.New(),
						NumAdmins: 1,
					}, nil)

				store.EXPECT().
					ListServerAdmins(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ServerAdmin{
						{
							ServerID: uuid.New(),
							UserID:   uuid.New(),
							AddedAt:  time.Now().UTC(),
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListServerAdminsResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
				require.Equal(t, handler.DefaultPageSize, res.Cursor.PageSize)
			},
		},
		{
			name: "page size is greater than allowed maximum",
			req: &pb.ListServerAdminsRequest{
				ServerId: uuid.New().String(),
				PageSize: &wrappers.Int32Value{
					Value: handler.MaxPageSize + 1,
				},
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:        uuid.New(),
						NumAdmins: 1,
					}, nil)

				store.EXPECT().
					ListServerAdmins(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ServerAdmin{}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListServerAdminsResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
				require.Equal(t, handler.DefaultPageSize, res.Cursor.PageSize)
			},
		},
		{
			name: "could not get server",
			req: &pb.ListServerAdminsRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, errors.New("server not found"))
			},
			checkResponse: func(t *testing.T, res *pb.ListServerAdminsResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not list server admins",
			req: &pb.ListServerAdminsRequest{
				ServerId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						ID:        uuid.New(),
						NumAdmins: 1,
					}, nil)

				store.EXPECT().
					ListServerAdmins(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ServerAdmin{}, errors.New("could not list server admins"))
			},
			checkResponse: func(t *testing.T, res *pb.ListServerAdminsResponse, err error) {
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

			tc.buildStubs(store)

			h := newTestHandler(store, cache, authService)

			res, err := h.ListServerAdmins(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
