package wordstore

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"gopkg.in/yaml.v3"
)

//go:embed data/default_words.yaml
var defaultWordsYAML []byte

var ErrServerNotfound = errors.New("server not found")

type DefaultWords struct {
	Words []Word `yaml:"words"`
}

type DefaultStore struct {
	words *[]Word
}

func NewDefaultStore() (Store, error) {
	var defaultWords DefaultWords
	if err := yaml.Unmarshal(defaultWordsYAML, &defaultWords); err != nil {
		return nil, fmt.Errorf("could not unmarshall default word list: %w", err)
	}

	return &DefaultStore{
		words: &defaultWords.Words,
	}, nil
}

func (ws *DefaultStore) GetRandomWords(
	ctx context.Context,
	dbStore db.Store,
	serverId uuid.UUID,
	numWords int,
) ([]Word, error) {
	server, err := dbStore.GetServerById(ctx, serverId)
	if err != nil {
		log.Error().Err(err).Str("server_id", serverId.String()).Msg("failed to get server from db")

		if errors.Is(err, db.RecordNotFoundError) {
			return nil, ErrServerNotfound
		}
		return nil, fmt.Errorf("failed to get server: %s from db: %w", serverId.String(), err)
	}

	numWordsInt32, err := commonUtil.IntToInt32(numWords)
	if err != nil {
		return nil, fmt.Errorf("failed to convert desired num random words to int32: %w", err)
	}

	if server.NumCustomWords > 0 {
		randomWords, err := dbStore.GetRandomWordsForServer(ctx, db.GetRandomWordsForServerParams{
			ServerID: serverId,
			N:        numWordsInt32,
		})
		if err != nil {
			log.Error().
				Err(err).
				Str("server_id", serverId.String()).Int("num_words", numWords).
				Msg("failed to get random words for server from db")
			return nil, fmt.Errorf("failed to get %d random words for server: %s from db: %w", numWords, serverId, err)
		}

		return mapFromDBWords(randomWords), nil
	}

	return getRandomWords(*ws.words, numWords)
}
