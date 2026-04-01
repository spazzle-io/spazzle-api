package server

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	mockservices "github.com/spazzle-io/spazzle-api/services/gameplay/internal/services/mock"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListUserServers(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.ListUserServersRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.ListUserServersResponse, err error)
	}{
		{
			name: "success",
			req: &pb.ListUserServersRequest{
				UserId: uuid.New().String(),
				PageSize: &wrapperspb.Int32Value{
					Value: handler.DefaultPageSize,
				},
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListUserServersRow{
						{
							ID:        uuid.New(),
							CreatedAt: time.Now().UTC(),
							StakePerGame: pgtype.Numeric{
								Int:   big.NewInt(12),
								Valid: true,
							},
							TotalVolume: pgtype.Numeric{
								Int:   big.NewInt(1000),
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetTotalUserServersCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListUserServersResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.NotEmpty(t, res.Servers)
				require.NotEmpty(t, res.Cursor)
				require.Equal(t, int64(1), res.TotalCount)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.ListUserServersRequest{
				UserId: "fake-id",
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.ListUserServersResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedViolations := []string{"userId"}
				handler.CheckInvalidRequestParams(t, err, expectedViolations)
			},
		},
		{
			name: "page size is zero",
			req: &pb.ListUserServersRequest{
				UserId:   uuid.New().String(),
				PageSize: &wrapperspb.Int32Value{Value: 0},
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListUserServersRow{
						{
							ID:        uuid.New(),
							CreatedAt: time.Now().UTC(),
							StakePerGame: pgtype.Numeric{
								Int:   big.NewInt(12),
								Valid: true,
							},
							TotalVolume: pgtype.Numeric{
								Int:   big.NewInt(1000),
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetTotalUserServersCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListUserServersResponse, err error) {
				require.NoError(t, err)
				require.Equal(t, res.Cursor.PageSize, handler.DefaultPageSize)
				require.NotEmpty(t, res)
			},
		},
		{
			name: "page size is greater than allowed maximum",
			req: &pb.ListUserServersRequest{
				UserId:   uuid.New().String(),
				PageSize: &wrapperspb.Int32Value{Value: handler.MaxPageSize + 1},
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListUserServersRow{
						{
							ID:        uuid.New(),
							CreatedAt: time.Now().UTC(),
							StakePerGame: pgtype.Numeric{
								Int:   big.NewInt(12),
								Valid: true,
							},
							TotalVolume: pgtype.Numeric{
								Int:   big.NewInt(1000),
								Valid: true,
							},
						},
					}, nil)

				store.EXPECT().
					GetTotalUserServersCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListUserServersResponse, err error) {
				require.NoError(t, err)
				require.Equal(t, res.Cursor.PageSize, handler.DefaultPageSize)
				require.NotEmpty(t, res)
			},
		},
		{
			name: "invalid after id",
			req: &pb.ListUserServersRequest{
				UserId:   uuid.New().String(),
				PageSize: &wrapperspb.Int32Value{Value: handler.DefaultPageSize},
				AfterId:  &wrapperspb.StringValue{Value: "abc"},
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.ListUserServersResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InvalidAfterIdError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not fetch user servers from db",
			req: &pb.ListUserServersRequest{
				UserId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListUserServersRow{}, errors.New("could not fetch user servers"))
			},
			checkResponse: func(t *testing.T, res *pb.ListUserServersResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not fetch user servers total count from db",
			req: &pb.ListUserServersRequest{
				UserId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListUserServersRow{}, nil)

				store.EXPECT().
					GetTotalUserServersCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), errors.New("could not fetch user servers total count"))
			},
			checkResponse: func(t *testing.T, res *pb.ListUserServersResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not map db user servers to pb",
			req: &pb.ListUserServersRequest{
				UserId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListUserServersRow{
						{
							ID:        uuid.New(),
							CreatedAt: time.Now().UTC(),
							StakePerGame: pgtype.Numeric{
								Valid: false,
							},
						},
					}, nil)

				store.EXPECT().
					GetTotalUserServersCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListUserServersResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "empty user server list from db",
			req: &pb.ListUserServersRequest{
				UserId: uuid.New().String(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListUserServersRow{}, nil)

				store.EXPECT().
					GetTotalUserServersCount(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListUserServersResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.Equal(t, res.Cursor.PageSize, handler.DefaultPageSize)
				require.Empty(t, res.Cursor.AfterId)
				require.Empty(t, res.Cursor.AfterCreatedAt)
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

			res, err := h.ListUserServers(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
