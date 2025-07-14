package handler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	mockdb "github.com/spazzle-io/spazzle-api/services/auth/internal/db/mock"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandler_VerifyAccessToken(t *testing.T) {
	wallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, wallet)

	userId := uuid.New()
	userIdStr := userId.String()

	testCases := []struct {
		name          string
		req           *pb.VerifyAccessTokenRequest
		buildContext  func(t *testing.T, tokenMaker token.Maker) context.Context
		checkResponse func(t *testing.T, res *pb.VerifyAccessTokenResponse, err error)
	}{
		{
			name: "success",
			req:  &pb.VerifyAccessTokenRequest{UserId: userIdStr},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return newContextWithBearerToken(
					t,
					userId,
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
			name: "invalid request arguments",
			req:  &pb.VerifyAccessTokenRequest{UserId: "invalid"},
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return newContextWithBearerToken(
					t,
					userId,
					wallet.Address,
					token.User,
					token.AccessToken,
					30*time.Second,
					tokenMaker)
			},
			checkResponse: func(t *testing.T, res *pb.VerifyAccessTokenResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"userId"}
				checkInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "missing authorization header",
			req:  &pb.VerifyAccessTokenRequest{UserId: userIdStr},
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
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			cache := mockcache.NewMockCache(ctrl)

			handler := newTestHandler(t, store, cache)

			ctx := tc.buildContext(t, handler.tokenMaker)
			res, err := handler.VerifyAccessToken(ctx, tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
