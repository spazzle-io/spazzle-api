package wordstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
)

type Word struct {
	Id       uuid.UUID `json:"id" yaml:"id"`
	Word     string    `json:"word" yaml:"word"`
	ServerID uuid.UUID `json:"server_id" yaml:"server_id"`
	AddedAt  time.Time `json:"added_at" yaml:"added_at"`
}

type Store interface {
	GetRandomWords(ctx context.Context, dbStore db.Store, serverId uuid.UUID, numWords int) ([]Word, error)
}
