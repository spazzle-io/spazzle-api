package token

import (
	"github.com/google/uuid"
	"time"
)

type Maker interface {
	CreateToken(userId uuid.UUID, walletAddress string, role Role, tokenAccess Type, duration time.Duration) (string, *Payload, error)
	VerifyToken(token string) (*Payload, error)
}
