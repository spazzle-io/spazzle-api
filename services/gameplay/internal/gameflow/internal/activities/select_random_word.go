package activities

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type SelectRandomWordParams struct {
	GameServerID uuid.UUID
}

type SelectRandomWordResult struct {
	Word string
}

func (a *Activities) SelectRandomWord(
	ctx context.Context,
	params SelectRandomWordParams,
) (*SelectRandomWordResult, error) {
	words, err := a.WordStore.GetRandomWords(ctx, a.Store, params.GameServerID, 1)
	if err != nil {
		return nil, err
	}

	if len(words) == 0 {
		return nil, errors.New("no words found")
	}

	return &SelectRandomWordResult{
		Word: words[0].Word,
	}, nil
}
