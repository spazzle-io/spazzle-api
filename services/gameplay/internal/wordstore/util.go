package wordstore

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"

	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
)

func mapFromDBWord(dbWord db.GetRandomWordsForServerRow) Word {
	return Word{
		Id:       dbWord.ID,
		Word:     dbWord.Word,
		ServerID: dbWord.ServerID,
		AddedAt:  dbWord.AddedAt,
	}
}

func mapFromDBWords(dbWords []db.GetRandomWordsForServerRow) []Word {
	words := make([]Word, 0, len(dbWords))

	for _, w := range dbWords {
		word := mapFromDBWord(w)

		if strings.TrimSpace(word.Word) != "" && !word.AddedAt.IsZero() {
			words = append(words, word)
		}
	}

	return words
}

func getRandomWords(words []Word, numWords int) ([]Word, error) {
	if numWords <= 0 || len(words) == 0 {
		return nil, nil
	}

	if numWords > len(words) {
		numWords = len(words)
	}

	w := append([]Word(nil), words...)

	for i := len(w) - 1; i > 0; i-- {
		j, err := cryptoInt(int64(i + 1))
		if err != nil {
			return nil, err
		}
		w[i], w[j] = w[j], w[i]
	}

	return w[:numWords], nil
}

// cryptoInt returns a uniform random int in [0, n).
func cryptoInt(n int64) (int, error) {
	if n <= 0 {
		return 0, errors.New("invalid range")
	}

	r, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		return 0, err
	}

	return int(r.Int64()), nil
}
