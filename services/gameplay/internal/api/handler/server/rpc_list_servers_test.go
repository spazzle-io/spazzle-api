package server

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

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

func TestListServers(t *testing.T) {
	testCases := []struct {
		name          string
		req           *pb.ListServersRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.ListServersResponse, err error)
	}{
		{
			name: "success",
			req: &pb.ListServersRequest{
				PageSize: defaultPageSize,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Server{
						{
							ID:        uuid.New(),
							CreatedAt: time.Now().UTC(),
							StakePerGame: pgtype.Numeric{
								Int:   big.NewInt(12),
								Valid: true,
							},
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListServersResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
			},
		},
		{
			name: "page size is zero",
			req: &pb.ListServersRequest{
				PageSize: 0,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Server{
						{
							ID:        uuid.New(),
							CreatedAt: time.Now().UTC(),
							StakePerGame: pgtype.Numeric{
								Int:   big.NewInt(12),
								Valid: true,
							},
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListServersResponse, err error) {
				require.NoError(t, err)
				require.Equal(t, res.Cursor.PageSize, defaultPageSize)
				require.NotEmpty(t, res)
			},
		},
		{
			name: "page size is greater than allowed maximum",
			req: &pb.ListServersRequest{
				PageSize: maxPageSize + 1,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Server{
						{
							ID:        uuid.New(),
							CreatedAt: time.Now().UTC(),
							StakePerGame: pgtype.Numeric{
								Int:   big.NewInt(12),
								Valid: true,
							},
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListServersResponse, err error) {
				require.NoError(t, err)
				require.Equal(t, res.Cursor.PageSize, defaultPageSize)
				require.NotEmpty(t, res)
			},
		},
		{
			name: "invalid after id",
			req: &pb.ListServersRequest{
				PageSize: defaultPageSize,
				AfterId:  "abc",
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, res *pb.ListServersResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, invalidAfterIdError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not fetch servers from db",
			req:  &pb.ListServersRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Server{}, errors.New("could not fetch servers"))
			},
			checkResponse: func(t *testing.T, res *pb.ListServersResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not map db servers to pb",
			req:  &pb.ListServersRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Server{
						{
							ID:        uuid.New(),
							CreatedAt: time.Now().UTC(),
							StakePerGame: pgtype.Numeric{
								Valid: false,
							},
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListServersResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, handler.InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "empty server list from db",
			req:  &pb.ListServersRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListServers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.Server{}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ListServersResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)

				require.Equal(t, res.Cursor.PageSize, defaultPageSize)
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

			res, err := h.ListServers(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
