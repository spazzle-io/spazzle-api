package handler

import (
	"context"
	"testing"

	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	mockdb "github.com/spazzle-io/spazzle-api/services/auth/internal/db/mock"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandler_GetSIWEPayload(t *testing.T) {
	wallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, wallet)

	testCases := []struct {
		name          string
		req           *pb.GetSIWEPayloadRequest
		buildStubs    func(cache *mockcache.MockCache)
		checkResponse func(t *testing.T, res *pb.GetSIWEPayloadResponse, err error)
	}{
		{
			name: "success",
			req: &pb.GetSIWEPayloadRequest{
				WalletAddress: wallet.Address,
				Domain:        "localhost",
				Uri:           "http://localhost:3000/login",
				ChainId:       2021,
			},
			buildStubs: func(cache *mockcache.MockCache) {
				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, res *pb.GetSIWEPayloadResponse, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, res)
			},
		},
		{
			name: "failed proto validation",
			req: &pb.GetSIWEPayloadRequest{
				WalletAddress: wallet.Address,
				Domain:        "localhost",
				Uri:           "http://localhost:3000/login",
			},
			buildStubs: func(cache *mockcache.MockCache) {},
			checkResponse: func(t *testing.T, res *pb.GetSIWEPayloadResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "wallet address is not hex",
			req: &pb.GetSIWEPayloadRequest{
				WalletAddress: "invalid_wallet_address",
				Domain:        "localhost",
				Uri:           "http://localhost:3000/login",
				ChainId:       2021,
			},
			buildStubs: func(cache *mockcache.MockCache) {},
			checkResponse: func(t *testing.T, res *pb.GetSIWEPayloadResponse, err error) {
				require.Error(t, err)
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
			tc.buildStubs(cache)

			handler := newTestHandler(t, store, cache)

			res, err := handler.GetSIWEPayload(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
