package token

import (
	"errors"
	"github.com/google/uuid"
	"time"
)

type Role string
type Type string

var ErrExpiredToken = errors.New("token is expired")

const (
	Admin Role = "admin"
	User  Role = "user"
)

const (
	AccessToken  Type = "Access Token"
	RefreshToken Type = "Refresh Token"
)

type Payload struct {
	ID            uuid.UUID `json:"id"`
	UserId        uuid.UUID `json:"user_id"`
	WalletAddress string    `json:"wallet_address"`
	Role          Role      `json:"role"`
	TokenType     Type      `json:"token_type"`
	IssuedAt      time.Time `json:"issued_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

func NewPayload(
	userId uuid.UUID,
	walletAddress string,
	role Role,
	tokenType Type,
	duration time.Duration) (*Payload, error) {
	tokenId, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	payload := &Payload{
		ID:            tokenId,
		UserId:        userId,
		WalletAddress: walletAddress,
		Role:          role,
		TokenType:     tokenType,
		IssuedAt:      time.Now().UTC(),
		ExpiresAt:     time.Now().UTC().Add(duration),
	}

	return payload, nil
}

func (payload *Payload) Valid() error {
	if time.Now().UTC().After(payload.ExpiresAt) {
		return ErrExpiredToken
	}

	return nil
}
