package handler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"github.com/stretchr/testify/require"
)

func TestHandler_VerifyAccessToken(t *testing.T) {
	wallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, wallet)

	testCases := []struct {
		name          string
		req           *pb.VerifyAccessTokenRequest
		buildContext  func(t *testing.T, tokenMaker token.Maker) context.Context
		checkResponse func(t *testing.T, res *pb.VerifyAccessTokenResponse, err error)
	}{
		{
			name: "success",
			req:  &pb.VerifyAccessTokenRequest{},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return newContextWithBearerToken(
					t,
					uuid.New(),
					wallet.Address,
					token.User,
					token.AccessToken,
					30*time.Second,
					tokenMaker)
			},
			checkResponse: func(t *testing.T, res *pb.VerifyAccessTokenResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
			},
		},
		{
			name: "missing authorization header",
			req:  &pb.VerifyAccessTokenRequest{},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return context.Background()
			},
			checkResponse: func(t *testing.T, res *pb.VerifyAccessTokenResponse, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, UnauthorizedAccessError)
				require.Empty(t, res)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newTestDeps(t)

			h := newTestHandler(deps)

			ctx := tc.buildContext(t, deps.tokenMaker)
			res, err := h.VerifyAccessToken(ctx, tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
