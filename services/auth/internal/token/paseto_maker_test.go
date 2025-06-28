package token

import (
	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func createPasetoToken(
	t *testing.T, role Role,
	tokenType Type,
	duration time.Duration) (*commonUtil.EthereumWallet, Maker, string) {
	ethereumWallet, err := commonUtil.NewEthereumWallet()
	require.NoError(t, err)
	require.NotEmpty(t, ethereumWallet)

	maker, err := NewPasetoMaker(gofakeit.LetterN(32))
	require.NoError(t, err)
	require.NotEmpty(t, maker)

	userId, err := uuid.NewRandom()
	require.NoError(t, err)
	require.NotEmpty(t, userId)

	token, payload, err := maker.CreateToken(userId, ethereumWallet.Address, role, tokenType, duration)
	require.NoError(t, err)
	require.NotEmpty(t, payload)

	require.NotEmpty(t, token)

	return ethereumWallet, maker, token
}

func TestNewPasetoMaker(t *testing.T) {
	testCases := []struct {
		name         string
		symmetricKey string
		isSuccess    bool
	}{
		{
			name:         "Success",
			symmetricKey: gofakeit.LetterN(32),
			isSuccess:    true,
		},
		{
			name:         "Invalid key size",
			symmetricKey: "invalid_symmetric_key",
			isSuccess:    false,
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			maker, err := NewPasetoMaker(tc.symmetricKey)
			if tc.isSuccess {
				require.NoError(t, err)
				require.NotEmpty(t, maker)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestPasetoMaker_CreateToken(t *testing.T) {
	createPasetoToken(t, Admin, AccessToken, 5*time.Hour)
}

func TestPasetoMaker_VerifyToken(t *testing.T) {
	duration := 5 * time.Hour
	wallet, maker, token := createPasetoToken(t, User, AccessToken, duration)

	testCases := []struct {
		name      string
		token     string
		isSuccess bool
	}{
		{
			name:      "Success",
			token:     token,
			isSuccess: true,
		},
		{
			name:      "Invalid token",
			token:     "invalid_token",
			isSuccess: false,
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			require.NotEmpty(t, maker)
			require.NotEmpty(t, tc.token)

			payload, err := maker.VerifyToken(tc.token)
			if tc.isSuccess {
				require.NoError(t, err)
				require.NotEmpty(t, payload)
			} else {
				require.Error(t, err)
				require.Nil(t, payload)
				return
			}

			require.NotEmpty(t, payload.ID)
			require.IsType(t, uuid.UUID{}, payload.ID)
			require.NotEmpty(t, payload.UserId)
			require.Equal(t, wallet.Address, payload.WalletAddress)
			require.Equal(t, User, payload.Role)
			require.WithinDuration(t, time.Now().UTC(), payload.IssuedAt, time.Second)
			require.WithinDuration(t, time.Now().UTC().Add(duration), payload.ExpiresAt, time.Second)
		})
	}
}
