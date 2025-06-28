package token

import (
	"github.com/google/uuid"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func createNewPayload(t *testing.T, role Role, tokenType Type, duration time.Duration) *Payload {
	ethereumWallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, ethereumWallet)

	userId, err := uuid.NewRandom()
	require.NoError(t, err)
	require.NotEmpty(t, userId)

	payload, err := NewPayload(userId, ethereumWallet.Address, role, tokenType, duration)
	require.NoError(t, err)
	require.NotEmpty(t, payload)

	require.NotEmpty(t, payload.ID)
	require.IsType(t, uuid.UUID{}, payload.ID)
	require.Equal(t, userId, payload.UserId)
	require.Equal(t, ethereumWallet.Address, payload.WalletAddress)
	require.Equal(t, tokenType, payload.TokenType)
	require.Equal(t, role, payload.Role)
	require.WithinDuration(t, time.Now().UTC(), payload.IssuedAt, time.Second)
	require.WithinDuration(t, time.Now().UTC().Add(duration), payload.ExpiresAt, time.Second)

	return payload
}

func TestNewPayload(t *testing.T) {
	payload := createNewPayload(t, User, AccessToken, 5*time.Hour)
	require.NotEmpty(t, payload)
}

func TestPayload_Valid(t *testing.T) {
	testCases := []struct {
		name            string
		duration        time.Duration
		isTokenValidErr error
	}{
		{
			name:            "Valid token",
			duration:        2 * time.Minute,
			isTokenValidErr: nil,
		},
		{
			name:            "Expired token",
			duration:        -1 * 5 * time.Hour,
			isTokenValidErr: ErrExpiredToken,
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			payload := createNewPayload(t, User, RefreshToken, tc.duration)
			isPayloadValidErr := payload.Valid()

			require.Equal(t, tc.isTokenValidErr, isPayloadValidErr)
		})
	}
}
