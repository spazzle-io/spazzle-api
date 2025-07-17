package db

import (
	"context"
	"crypto/rand"
	"math/big"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/stretchr/testify/require"
)

func createTestSession(t *testing.T, userId uuid.UUID, walletAddress string) Session {
	maker, err := token.NewPasetoMaker(gofakeit.LetterN(32))
	require.NoError(t, err)
	require.NotEmpty(t, maker)

	refreshToken, payload, err := maker.CreateToken(userId, walletAddress, token.User, token.RefreshToken, 1*time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, payload)
	require.NotEmpty(t, refreshToken)

	randomNum, err := rand.Int(rand.Reader, big.NewInt(2))
	require.NoError(t, err)

	// Set client IP to either IPv4 or IPv6
	clientIP := gofakeit.IPv4Address()
	if randomNum.Int64() == 1 {
		clientIP = gofakeit.IPv6Address()
	}

	params := CreateSessionParams{
		ID:            uuid.New(),
		UserID:        userId,
		WalletAddress: walletAddress,
		RefreshToken:  refreshToken,
		UserAgent:     gofakeit.UserAgent(),
		ClientIp:      clientIP,
		ExpiresAt:     time.Now().UTC().Add(3 * time.Hour),
	}

	session, err := testStore.CreateSession(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, session)

	require.Equal(t, params.ID, session.ID)
	require.Equal(t, params.UserID, session.UserID)
	require.Equal(t, params.WalletAddress, session.WalletAddress)
	require.Equal(t, params.RefreshToken, session.RefreshToken)
	require.Equal(t, params.ClientIp, session.ClientIp)
	require.WithinDuration(t, params.ExpiresAt, session.ExpiresAt, time.Second)
	require.NotZero(t, session.CreatedAt)

	require.Equal(t, userId, session.UserID)
	require.Equal(t, walletAddress, session.WalletAddress)

	require.False(t, session.IsRevoked)

	return session
}

func TestCreateSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userId, wallet, credential := createTestCredential(t)
	require.NotEmpty(t, userId)
	require.NotEmpty(t, wallet)
	require.NotEmpty(t, credential)

	session := createTestSession(t, userId, wallet.Address)
	require.NotEmpty(t, session)
}

func TestGetSessionById(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userId, wallet, credential := createTestCredential(t)
	require.NotEmpty(t, userId)
	require.NotEmpty(t, wallet)
	require.NotEmpty(t, credential)

	session := createTestSession(t, userId, wallet.Address)
	require.NotEmpty(t, session)

	fetchedSession, err := testStore.GetSessionById(context.Background(), session.ID)
	require.NoError(t, err)
	require.NotEmpty(t, fetchedSession)

	require.Equal(t, session.ID, fetchedSession.ID)
	require.Equal(t, session.UserID, fetchedSession.UserID)
	require.Equal(t, session.WalletAddress, fetchedSession.WalletAddress)
	require.Equal(t, session.RefreshToken, fetchedSession.RefreshToken)
	require.Equal(t, session.UserAgent, fetchedSession.UserAgent)
	require.Equal(t, session.ClientIp, fetchedSession.ClientIp)
	require.Equal(t, session.IsRevoked, fetchedSession.IsRevoked)
	require.WithinDuration(t, session.ExpiresAt, fetchedSession.ExpiresAt, time.Second)
	require.WithinDuration(t, session.CreatedAt, fetchedSession.CreatedAt, time.Second)

	require.False(t, fetchedSession.IsRevoked)
}

func TestRevokeAccountSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	userId, wallet, credential := createTestCredential(t)
	require.NotEmpty(t, userId)
	require.NotEmpty(t, wallet)
	require.NotEmpty(t, credential)

	numCreatedSessions := 12
	var createdSessions []Session

	for i := 0; i < numCreatedSessions; i++ {
		session := createTestSession(t, userId, wallet.Address)
		require.NotEmpty(t, session)

		createdSessions = append(createdSessions, session)
	}

	commandTag, err := testStore.RevokeSessions(context.Background(), userId)
	require.NoError(t, err)
	require.NotEmpty(t, commandTag)
	require.Equal(t, numCreatedSessions, int(commandTag.RowsAffected()))

	for _, createdSession := range createdSessions {
		gotSession, err := testStore.GetSessionById(context.Background(), createdSession.ID)
		require.NoError(t, err)
		require.NotEmpty(t, gotSession)
		require.True(t, gotSession.IsRevoked)
	}
}
