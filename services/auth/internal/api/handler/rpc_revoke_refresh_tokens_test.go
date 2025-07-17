package handler

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	mockdb "github.com/spazzle-io/spazzle-api/services/auth/internal/db/mock"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRevokeRefreshTokens(t *testing.T) {
	userId := uuid.New()

	testCases := []struct {
		name          string
		req           *pb.RevokeRefreshTokensRequest
		buildStubs    func(store *mockdb.MockStore, cache *mockcache.MockCache)
		buildContext  func(t *testing.T, tokenMaker token.Maker) context.Context
		checkResponse func(t *testing.T, res *pb.RevokeRefreshTokensResponse, err error)
	}{
		{
			name: "success",
			req: &pb.RevokeRefreshTokensRequest{
				UserId: userId.String(),
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				testCt := pgconn.NewCommandTag(fmt.Sprintf("test %d", int64(2)))

				store.EXPECT().
					RevokeSessions(gomock.Any(), userId).
					Times(1).
					Return(testCt, nil)
			},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return newContextWithBearerToken(
					t, userId, "walletAddress", token.User, token.AccessToken, 30*time.Second, tokenMaker,
				)
			},
			checkResponse: func(t *testing.T, res *pb.RevokeRefreshTokensResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.RevokeRefreshTokensRequest{
				UserId: "",
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return context.Background()
			},
			checkResponse: func(t *testing.T, res *pb.RevokeRefreshTokensResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"userId"}
				checkInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "missing access token",
			req: &pb.RevokeRefreshTokensRequest{
				UserId: userId.String(),
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return context.Background()
			},
			checkResponse: func(t *testing.T, res *pb.RevokeRefreshTokensResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not revoke sessions",
			req: &pb.RevokeRefreshTokensRequest{
				UserId: userId.String(),
			},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					RevokeSessions(gomock.Any(), userId).
					Times(1).
					Return(pgconn.CommandTag{}, errors.New("some db error"))
			},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return newContextWithBearerToken(
					t, userId, "walletAddress", token.User, token.AccessToken, 30*time.Second, tokenMaker,
				)
			},
			checkResponse: func(t *testing.T, res *pb.RevokeRefreshTokensResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cache := mockcache.NewMockCache(ctrl)
			store := mockdb.NewMockStore(ctrl)

			handler := newTestHandler(t, store, cache)
			tc.buildStubs(store, cache)

			ctx := tc.buildContext(t, handler.tokenMaker)
			res, err := handler.RevokeRefreshTokens(ctx, tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
