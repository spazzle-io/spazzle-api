package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	mockservices "github.com/spazzle-io/spazzle-api/services/users/internal/services/mock"
	"github.com/spazzle-io/spazzle-api/services/users/internal/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAuthorizeUser(t *testing.T) {
	userId, err := uuid.NewRandom()
	require.NoError(t, err)
	require.NotNil(t, userId)

	testCases := []struct {
		name          string
		buildStubs    func(ctx context.Context, authService *mockservices.MockAuthGrpcService)
		userId        uuid.UUID
		checkResponse func(t *testing.T, payload *pb.AccessTokenPayload, err error)
	}{
		{
			name: "success",
			buildStubs: func(ctx context.Context, authService *mockservices.MockAuthGrpcService) {
				expectedPayload := &pb.VerifyAccessTokenRequest{
					UserId: userId.String(),
				}

				response := &pb.VerifyAccessTokenResponse{
					AccessTokenPayload: &pb.AccessTokenPayload{
						Id:            "some-id",
						UserId:        userId.String(),
						WalletAddress: "0xc0ffee254729296a45a3885639AC7E10F9d54979",
						Role:          pb.AccessTokenPayload_ROLE_USER,
					},
				}

				authService.EXPECT().
					VerifyAccessToken(ctx, "test-service", expectedPayload).
					Times(1).
					Return(response, nil)
			},
			userId: userId,
			checkResponse: func(t *testing.T, payload *pb.AccessTokenPayload, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				require.Equal(t, payload.Id, "some-id")
				require.Equal(t, payload.WalletAddress, "0xc0ffee254729296a45a3885639AC7E10F9d54979")
				require.Equal(t, payload.Role, pb.AccessTokenPayload_ROLE_USER)
			},
		},
		{
			name: "invalid access token",
			buildStubs: func(ctx context.Context, authService *mockservices.MockAuthGrpcService) {
				expectedPayload := &pb.VerifyAccessTokenRequest{
					UserId: userId.String(),
				}

				authService.EXPECT().
					VerifyAccessToken(ctx, "test-service", expectedPayload).
					Times(1).
					Return(nil, errors.New("invalid access token"))
			},
			userId: userId,
			checkResponse: func(t *testing.T, payload *pb.AccessTokenPayload, err error) {
				require.Error(t, err)
				require.Nil(t, payload)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			config := util.Config{
				ServiceName: "test-service",
			}

			authService := mockservices.NewMockAuthGrpcService(ctrl)

			testCase.buildStubs(context.Background(), authService)

			payload, err := AuthorizeUser(context.Background(), testCase.userId, config, authService)
			testCase.checkResponse(t, payload, err)
		})
	}
}
