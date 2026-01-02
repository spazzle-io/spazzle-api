package gameserver

import (
	"errors"
	"fmt"
	"slices"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
)

const wordChoicesCachePrefix = "word_choices"

var (
	ErrNoCachedWords    = errors.New("no cached words")
	ErrWordNotInChoices = errors.New("word not in choices")
)

func (gs *GameServer) getWordChoices() ([]string, error) {
	server, err := gs.Store.GetServerById(gs.ctx, gs.serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server by id: %w", err)
	}

	// TODO: Add API validation for fields like NumDrawingOptions when creating/updating server to prevent erroneous input

	serverWords, err := gs.WordStore.GetRandomWords(gs.ctx, gs.Store, gs.serverID, int(server.NumDrawingOptions))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch server random words: %w", err)
	}

	words := make([]string, 0, server.NumDrawingOptions)
	for _, serverWord := range serverWords {
		words = append(words, serverWord.Word)
	}

	cacheKey := gs.getWordChoicesCacheKey()
	if err = gs.Cache.Set(gs.ctx, cacheKey, words, types.WordSelectionTimeout); err != nil {
		return nil, fmt.Errorf("failed to cache words: %w", err)
	}

	return words, nil
}

func (gs *GameServer) chooseWord(word string) error {
	cacheKey := gs.getWordChoicesCacheKey()

	var words []string
	err := gs.Cache.Get(gs.ctx, cacheKey, &words)
	if err != nil {
		if errors.Is(err, commonCache.ErrKeyNotFound) {
			return ErrNoCachedWords
		}

		return fmt.Errorf("failed to get cached words: %w", err)
	}

	if !slices.Contains(words, word) {
		return ErrWordNotInChoices
	}

	if err = gs.GfClient.SelectWord(gs.serverID, word, gs.getWorkflowSelectionID()); err != nil {
		return fmt.Errorf("failed to select word: %w", err)
	}

	return nil
}

func (gs *GameServer) getWordChoicesCacheKey() string {
	return fmt.Sprintf("%s-%s:%s:%s:%s",
		gs.Env.ServiceName,
		wordChoicesCachePrefix,
		gs.serverID.String(),
		gs.getGameID().String(),
		gs.getCurrentArtist().String(),
	)
}
