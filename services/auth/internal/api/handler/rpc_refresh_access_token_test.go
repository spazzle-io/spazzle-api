package handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	mockdb "github.com/spazzle-io/spazzle-api/services/auth/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRefreshAccessToken(t *testing.T) {
	wallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, wallet)

	testCases := []struct {
		name          string
		req           *pb.RefreshAccessTokenRequest
		buildStubs    func(store *mockdb.MockStore, cache *mockcache.MockCache)
		buildContext  func(t *testing.T, tokenMaker token.Maker) context.Context
		checkResponse func(t *testing.T, res *pb.RefreshAccessTokenResponse, err error)
	}{
		{
			name: "success",
			req:  &pb.RefreshAccessTokenRequest{},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{
						ID:           uuid.New(),
						IsRevoked:    false,
						RefreshToken: "refresh_token",
						ExpiresAt:    time.Now().UTC().Add(1 * time.Minute),
					}, nil)
			},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return newContextWithBearerToken(
					t, uuid.New(), wallet.Address, token.User, token.RefreshToken, 30*time.Second, tokenMaker,
				)
			},
			checkResponse: func(t *testing.T, res *pb.RefreshAccessTokenResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
			},
		},
		{
			name:       "no refresh token",
			req:        &pb.RefreshAccessTokenRequest{},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return context.Background()
			},
			checkResponse: func(t *testing.T, res *pb.RefreshAccessTokenResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
		{
			name: "could not get session",
			req:  &pb.RefreshAccessTokenRequest{},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{}, errors.New("some error"))
			},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return newContextWithBearerToken(
					t, uuid.New(), wallet.Address, token.User, token.RefreshToken, 30*time.Second, tokenMaker,
				)
			},
			checkResponse: func(t *testing.T, res *pb.RefreshAccessTokenResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, InternalServerError)
				require.Empty(t, res)
			},
		},
		{
			name: "session is revoked",
			req:  &pb.RefreshAccessTokenRequest{},
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{
						ID:        uuid.New(),
						IsRevoked: true,
					}, nil)
			},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return newContextWithBearerToken(
					t, uuid.New(), wallet.Address, token.User, token.RefreshToken, 30*time.Second, tokenMaker,
				)
			},
			checkResponse: func(t *testing.T, res *pb.RefreshAccessTokenResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
	}

	for _, testCase := range testCases {
		tc := testCase

		t.Run(testCase.name, func(t *testing.T) {
			deps := newTestDeps(t)

			h := newTestHandler(deps)

			tc.buildStubs(deps.store, deps.cache)
			ctx := tc.buildContext(t, deps.tokenMaker)

			res, err := h.RefreshAccessToken(ctx, tc.req)
			testCase.checkResponse(t, res, err)
		})
	}
}
