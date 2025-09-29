package wordstore

import (
	"testing"
	"time"

	"github.com/google/uuid"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/stretchr/testify/require"
)

func TestMapFromBDWord(t *testing.T) {
	wordId := uuid.New()
	serverId := uuid.New()
	wordAddedAt := time.Now().UTC()

	testCases := []struct {
		name         string
		dbWord       db.GetRandomWordsForServerRow
		expectedWord Word
	}{
		{
			name: "success",
			dbWord: db.GetRandomWordsForServerRow{
				ID:       wordId,
				Word:     "some-word",
				ServerID: serverId,
				AddedAt:  wordAddedAt,
			},
			expectedWord: Word{
				Id:       wordId,
				Word:     "some-word",
				ServerID: serverId,
				AddedAt:  wordAddedAt,
			},
		},
		{
			name:         "empty db word",
			dbWord:       db.GetRandomWordsForServerRow{},
			expectedWord: Word{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			word := mapFromDBWord(tc.dbWord)
			require.Equal(t, tc.expectedWord, word)
		})
	}
}

func TestMapFromBDWords(t *testing.T) {
	wordId := uuid.New()
	serverId := uuid.New()
	wordAddedAt := time.Now().UTC()

	dbWords := []db.GetRandomWordsForServerRow{
		{
			ID:       wordId,
			Word:     "some-word",
			ServerID: serverId,
			AddedAt:  wordAddedAt,
		},
		{
			ID:       wordId,
			Word:     "another-word",
			ServerID: serverId,
			AddedAt:  wordAddedAt,
		},
		{},
	}

	expectedWords := []Word{
		{
			Id:       wordId,
			Word:     "some-word",
			ServerID: serverId,
			AddedAt:  wordAddedAt,
		},
		{
			Id:       wordId,
			Word:     "another-word",
			ServerID: serverId,
			AddedAt:  wordAddedAt,
		},
	}

	words := mapFromDBWords(dbWords)
	require.Equal(t, expectedWords, words)
}

func TestUtilGetRandomWords(t *testing.T) {
	words := []Word{
		{
			Id:       uuid.New(),
			Word:     "some-word",
			ServerID: uuid.New(),
			AddedAt:  time.Now().UTC(),
		},
		{
			Id:       uuid.New(),
			Word:     "another-word",
			ServerID: uuid.New(),
			AddedAt:  time.Now().UTC(),
		},
		{
			Id:       uuid.New(),
			Word:     "one-more",
			ServerID: uuid.New(),
			AddedAt:  time.Now().UTC(),
		},
		{
			Id:       uuid.New(),
			Word:     "jk",
			ServerID: uuid.New(),
			AddedAt:  time.Now().UTC(),
		},
	}

	testCases := []struct {
		name             string
		words            []Word
		numWords         int
		expectedNumWords int
	}{
		{
			name:             "success",
			words:            words,
			numWords:         len(words),
			expectedNumWords: len(words),
		},
		{
			name:             "less words than provided words",
			words:            words,
			numWords:         2,
			expectedNumWords: 2,
		},
		{
			name:             "more words than provided words",
			words:            words,
			numWords:         len(words) + 2,
			expectedNumWords: len(words),
		},
		{
			name:             "numWords <= 0",
			words:            words,
			numWords:         0,
			expectedNumWords: 0,
		},
		{
			name:             "empty word list",
			words:            []Word{},
			numWords:         2,
			expectedNumWords: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			randomWords, err := getRandomWords(tc.words, tc.numWords)
			require.NoError(t, err)
			require.Equal(t, tc.expectedNumWords, len(randomWords))
		})
	}
}
