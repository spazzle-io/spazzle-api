package middleware

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func getTestTokenMaker(t *testing.T) token.Maker {
	tokenSymmetricKey := gofakeit.LetterN(32)

	tokenMaker, err := token.NewPasetoMaker(tokenSymmetricKey)
	require.NoError(t, err)
	require.NotEmpty(t, tokenMaker)

	return tokenMaker
}

func TestAuthorize(t *testing.T) {
	testUserId := uuid.New()
	testWalletAddress := "0x37E0D2456f58fDe5bfe56B0790591b3b8181c42E"

	testCases := []struct {
		name            string
		buildContext    func(t *testing.T, tokenMaker token.Maker) context.Context
		userId          uuid.UUID
		tokenType       token.Type
		authorizedRoles []token.Role
		checkResponse   func(t *testing.T, payload *token.Payload, err error)
	}{
		{
			name: "Success",
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				tk, _, err := tokenMaker.CreateToken(testUserId, testWalletAddress, token.User, token.AccessToken, 30*time.Second)
				require.NoError(t, err)
				require.NotEmpty(t, tk)

				bearerToken := fmt.Sprintf("%s %s", authorizationBearer, tk)
				md := metadata.MD{
					authorizationHeader: []string{
						bearerToken,
					},
				}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			userId:          testUserId,
			tokenType:       token.AccessToken,
			authorizedRoles: []token.Role{token.User},
			checkResponse: func(t *testing.T, payload *token.Payload, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				require.NotZero(t, payload.ID)

				require.Equal(t, testUserId, payload.UserId)
				require.Equal(t, testWalletAddress, payload.WalletAddress)
				require.Equal(t, token.AccessToken, payload.TokenType)
				require.Equal(t, token.User, payload.Role)

				require.WithinDuration(t, time.Now().UTC(), payload.IssuedAt, time.Second)
				require.WithinDuration(t, time.Now().UTC().Add(30*time.Second), payload.ExpiresAt, time.Second)
			},
		},
		{
			name: "Failure - missing metadata from incoming context",
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return context.Background()
			},
			userId:          testUserId,
			tokenType:       token.AccessToken,
			authorizedRoles: []token.Role{token.User},
			checkResponse: func(t *testing.T, payload *token.Payload, err error) {
				require.Error(t, err)
				require.Empty(t, payload)
			},
		},
		{
			name: "Failure - missing authorization header",
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				md := metadata.MD{
					"some_other_header": []string{
						"some_value",
					},
				}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			userId:          testUserId,
			tokenType:       token.AccessToken,
			authorizedRoles: []token.Role{token.User},
			checkResponse: func(t *testing.T, payload *token.Payload, err error) {
				require.Error(t, err)
				require.Empty(t, payload)
			},
		},
		{
			name: "Failure - invalid authorization header format",
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				md := metadata.MD{
					authorizationHeader: []string{
						"some_value",
					},
				}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			userId:          testUserId,
			tokenType:       token.AccessToken,
			authorizedRoles: []token.Role{token.User},
			checkResponse: func(t *testing.T, payload *token.Payload, err error) {
				require.Error(t, err)
				require.Empty(t, payload)
			},
		},
		{
			name: "Failure - unsupported authorization type",
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				md := metadata.MD{
					authorizationHeader: []string{
						fmt.Sprintf("%s %s", "unsupported_auth_type", "some_token"),
					},
				}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			userId:          testUserId,
			tokenType:       token.AccessToken,
			authorizedRoles: []token.Role{token.User},
			checkResponse: func(t *testing.T, payload *token.Payload, err error) {
				require.Error(t, err)
				require.Empty(t, payload)
			},
		},
		{
			name: "Failure - invalid authorization token",
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				tk, _, err := tokenMaker.CreateToken(testUserId, testWalletAddress, token.User, token.AccessToken, -30*time.Second)
				require.NoError(t, err)

				md := metadata.MD{
					authorizationHeader: []string{
						fmt.Sprintf("%s %s", authorizationBearer, tk),
					},
				}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			userId:          testUserId,
			tokenType:       token.AccessToken,
			authorizedRoles: []token.Role{token.User},
			checkResponse: func(t *testing.T, payload *token.Payload, err error) {
				require.Error(t, err)
				require.Empty(t, payload)
			},
		},
		{
			name: "Failure - unauthorized role",
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				tk, _, err := tokenMaker.CreateToken(testUserId, testWalletAddress, token.Admin, token.AccessToken, 30*time.Second)
				require.NoError(t, err)

				md := metadata.MD{
					authorizationHeader: []string{
						fmt.Sprintf("%s %s", authorizationBearer, tk),
					},
				}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			userId:          testUserId,
			tokenType:       token.AccessToken,
			authorizedRoles: []token.Role{token.User},
			checkResponse: func(t *testing.T, payload *token.Payload, err error) {
				require.Error(t, err)
				require.Empty(t, payload)
			},
		},
		{
			name: "Failure - mismatch in user id for user role",
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				tk, _, err := tokenMaker.CreateToken(testUserId, testWalletAddress, token.User, token.AccessToken, 30*time.Second)
				require.NoError(t, err)

				md := metadata.MD{
					authorizationHeader: []string{
						fmt.Sprintf("%s %s", authorizationBearer, tk),
					},
				}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			userId:          uuid.New(),
			tokenType:       token.AccessToken,
			authorizedRoles: []token.Role{token.User},
			checkResponse: func(t *testing.T, payload *token.Payload, err error) {
				require.Error(t, err)
				require.Empty(t, payload)
			},
		},
		{
			name: "Failure - mismatch in token access",
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				tk, _, err := tokenMaker.CreateToken(testUserId, testWalletAddress, token.User, token.AccessToken, 30*time.Second)
				require.NoError(t, err)

				md := metadata.MD{
					authorizationHeader: []string{
						fmt.Sprintf("%s %s", authorizationBearer, tk),
					},
				}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			userId:          testUserId,
			tokenType:       token.RefreshToken,
			authorizedRoles: []token.Role{token.User},
			checkResponse: func(t *testing.T, payload *token.Payload, err error) {
				require.Error(t, err)
				require.Empty(t, payload)
			},
		},
		{
			name: "Success: admin privileged access",
			buildContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				tk, _, err := tokenMaker.CreateToken(testUserId, testWalletAddress, token.Admin, token.AccessToken, 30*time.Second)
				require.NoError(t, err)

				md := metadata.MD{
					authorizationHeader: []string{
						fmt.Sprintf("%s %s", authorizationBearer, tk),
					},
				}

				return metadata.NewIncomingContext(context.Background(), md)
			},
			userId:          uuid.New(),
			tokenType:       token.AccessToken,
			authorizedRoles: []token.Role{token.User, token.Admin},
			checkResponse: func(t *testing.T, payload *token.Payload, err error) {
				require.NoError(t, err)
				require.NotEmpty(t, payload)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			tokenMaker := getTestTokenMaker(t)

			ctx := tc.buildContext(t, tokenMaker)
			payload, err := Authorize(ctx, tc.userId, tokenMaker, tc.tokenType, tc.authorizedRoles)

			tc.checkResponse(t, payload, err)
		})
	}
}
