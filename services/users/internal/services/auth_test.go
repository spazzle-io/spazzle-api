package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestAuthServiceGrpcClient_withMetadata(t *testing.T) {
	testCases := []struct {
		name                              string
		buildContext                      func(t *testing.T) context.Context
		serviceAuthenticationPayload      string
		serviceAuthenticationPayloadError error
		checkResponse                     func(t *testing.T, ctx context.Context, err error)
	}{
		{
			name: "could not fetch incoming metadata",
			buildContext: func(t *testing.T) context.Context {
				return context.Background()
			},
			serviceAuthenticationPayload:      "",
			serviceAuthenticationPayloadError: errors.New("some error"),
			checkResponse: func(t *testing.T, ctx context.Context, err error) {
				require.Error(t, err)
				require.Nil(t, ctx)
			},
		},
		{
			name: "could not get service authentication payload",
			buildContext: func(t *testing.T) context.Context {
				md := metadata.MD{}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			serviceAuthenticationPayload:      "",
			serviceAuthenticationPayloadError: errors.New("some error"),
			checkResponse: func(t *testing.T, ctx context.Context, err error) {
				require.Error(t, err)
				require.Nil(t, ctx)
			},
		},
		{
			name: "without metadata in incoming context",
			buildContext: func(t *testing.T) context.Context {
				md := metadata.MD{}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			serviceAuthenticationPayload:      "some-authentication-payload",
			serviceAuthenticationPayloadError: nil,
			checkResponse: func(t *testing.T, ctx context.Context, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, ctx)

				md, ok := metadata.FromOutgoingContext(ctx)
				require.True(t, ok)

				serviceAuthenticationHeaderVals := md.Get(commonMiddleware.XServiceAuthenticationHeader)
				require.NotEmpty(t, serviceAuthenticationHeaderVals)

				authorizationHeaderVals := md.Get(commonMiddleware.AuthorizationHeader)
				require.Empty(t, authorizationHeaderVals)
			},
		},
		{
			name: "with populated metadata in incoming context",
			buildContext: func(t *testing.T) context.Context {
				md := metadata.MD{commonMiddleware.AuthorizationHeader: []string{
					"some-auth-header-val",
				}, commonMiddleware.XForwardedForHeader: []string{
					"some-xff-header-val",
				}, commonMiddleware.UserAgentHeader: []string{
					"some-user-agent-header-val",
				}}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			serviceAuthenticationPayload:      "some-authentication-payload",
			serviceAuthenticationPayloadError: nil,
			checkResponse: func(t *testing.T, ctx context.Context, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, ctx)

				md, ok := metadata.FromOutgoingContext(ctx)
				require.True(t, ok)

				serviceAuthenticationHeaderVals := md.Get(commonMiddleware.XServiceAuthenticationHeader)
				require.NotEmpty(t, serviceAuthenticationHeaderVals[0])
				require.Equal(t, "some-authentication-payload", serviceAuthenticationHeaderVals[0])

				authorizationHeaderVals := md.Get(commonMiddleware.AuthorizationHeader)
				require.NotEmpty(t, authorizationHeaderVals[0])
				require.Equal(t, "some-auth-header-val", authorizationHeaderVals[0])

				xForwardedForHeaderVals := md.Get(commonMiddleware.XForwardedForHeader)
				require.NotEmpty(t, xForwardedForHeaderVals[0])
				require.Equal(t, "some-xff-header-val", xForwardedForHeaderVals[0])

				userAgentHeaderVals := md.Get(commonMiddleware.UserAgentHeader)
				require.NotEmpty(t, userAgentHeaderVals[0])
				require.Equal(t, "some-user-agent-header-val", userAgentHeaderVals[0])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			initialGetServiceAuthenticationPayload := getServiceAuthenticationPayload
			getServiceAuthenticationPayload = func(serviceName string) (string, error) {
				return tc.serviceAuthenticationPayload, tc.serviceAuthenticationPayloadError
			}
			defer func() {
				getServiceAuthenticationPayload = initialGetServiceAuthenticationPayload
			}()

			ctx := tc.buildContext(t)

			client, err := NewAuthServiceGrpcClient("some-server-address")
			require.NoError(t, err)
			require.NotEmpty(t, client)

			defer func() {
				err := client.(*AuthServiceGrpcClient).Close()
				require.NoError(t, err)
			}()

			ctx, err = client.(*AuthServiceGrpcClient).withMetadata(ctx, "test")
			tc.checkResponse(t, ctx, err)
		})
	}
}

type mockAuthClientHandler struct {
	IsVerifyAccessTokenCalled bool
	IsAuthenticateCalled      bool
}

func (h *mockAuthClientHandler) GetSIWEPayload(_ context.Context, _ *pb.GetSIWEPayloadRequest, _ ...grpc.CallOption) (*pb.GetSIWEPayloadResponse, error) {
	return nil, nil
}

func (h *mockAuthClientHandler) Authenticate(_ context.Context, _ *pb.AuthenticateRequest, _ ...grpc.CallOption) (*pb.AuthenticateResponse, error) {
	h.IsAuthenticateCalled = true
	return nil, nil
}

func (h *mockAuthClientHandler) VerifyAccessToken(_ context.Context, _ *pb.VerifyAccessTokenRequest, _ ...grpc.CallOption) (*pb.VerifyAccessTokenResponse, error) {
	h.IsVerifyAccessTokenCalled = true
	return nil, nil
}

func (h *mockAuthClientHandler) RefreshAccessToken(_ context.Context, _ *pb.RefreshAccessTokenRequest, _ ...grpc.CallOption) (*pb.RefreshAccessTokenResponse, error) {
	return nil, nil
}

func (h *mockAuthClientHandler) RevokeRefreshTokens(_ context.Context, _ *pb.RevokeRefreshTokensRequest, _ ...grpc.CallOption) (*pb.RevokeRefreshTokensResponse, error) {
	return nil, nil
}

func TestAuthServiceGrpcClient_VerifyAccessToken(t *testing.T) {
	dummyClient := &mockAuthClientHandler{}

	mockAuthServiceGrpcClient := &AuthServiceGrpcClient{
		client: dummyClient,
	}

	initialGetServiceAuthenticationPayload := getServiceAuthenticationPayload
	getServiceAuthenticationPayload = func(serviceName string) (string, error) {
		return "some authentication payload", nil
	}
	defer func() {
		getServiceAuthenticationPayload = initialGetServiceAuthenticationPayload
	}()

	md := metadata.MD{}
	ctx := metadata.NewIncomingContext(context.Background(), md)

	payload := &pb.VerifyAccessTokenRequest{}

	_, err := mockAuthServiceGrpcClient.VerifyAccessToken(ctx, "test", payload)
	require.NoError(t, err)

	require.True(t, dummyClient.IsVerifyAccessTokenCalled)
}

func TestAuthServiceGrpcClient_Authenticate(t *testing.T) {
	dummyClient := &mockAuthClientHandler{}

	mockAuthServiceGrpcClient := &AuthServiceGrpcClient{
		client: dummyClient,
	}

	initialGetServiceAuthenticationPayload := getServiceAuthenticationPayload
	getServiceAuthenticationPayload = func(serviceName string) (string, error) {
		return "some authentication payload", nil
	}
	defer func() {
		getServiceAuthenticationPayload = initialGetServiceAuthenticationPayload
	}()

	md := metadata.MD{}
	ctx := metadata.NewIncomingContext(context.Background(), md)

	payload := &pb.AuthenticateRequest{
		WalletAddress: "some-wallet-address",
		UserId:        uuid.New().String(),
		Signature:     "some-signature",
	}

	_, err := mockAuthServiceGrpcClient.Authenticate(ctx, "test", payload)
	require.NoError(t, err)

	require.True(t, dummyClient.IsAuthenticateCalled)
}
