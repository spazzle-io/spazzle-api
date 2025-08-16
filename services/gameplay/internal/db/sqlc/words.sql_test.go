package db

import (
	"context"
	"math/rand"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestAddWordsToServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	words := []string{gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun()}
	params := AddWordsToServerParams{
		ServerID: server.ID,
		Words:    words,
	}

	res, err := testStore.AddWordsToServer(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, res)
	require.Equal(t, int64(4), res.RowsAffected())

	// ensure we can't have duplicates but db handles it silently
	res, err = testStore.AddWordsToServer(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, res)
	require.Equal(t, int64(0), res.RowsAffected())
}

func TestListWords(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	lastSeenWordIdx := 0
	words := []string{gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun()}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	for _, word := range words {
		addWordsParams := AddWordsToServerParams{
			ServerID: server.ID,
			Words:    []string{word},
		}

		resAddWords, err := testStore.AddWordsToServer(context.Background(), addWordsParams)
		require.NoError(t, err)
		require.NotEmpty(t, resAddWords)
	}

	firstPageParams := ListWordsParams{
		ServerID: server.ID,
		PageSize: 2,
	}
	firstPageWords, err := testStore.ListWords(context.Background(), firstPageParams)
	require.NoError(t, err)
	require.NotEmpty(t, firstPageWords)
	require.Len(t, firstPageWords, 2)

	for _, word := range firstPageWords {
		require.Equal(t, words[len(words)-lastSeenWordIdx-1], word.Word)
		lastSeenWordIdx += 1
	}

	lastPageParams := ListWordsParams{
		ServerID: server.ID,
		PageSize: 5,
		AfterAddedAt: pgtype.Timestamptz{
			Time:  firstPageWords[len(firstPageWords)-1].AddedAt,
			Valid: true,
		},
		AfterID: pgtype.UUID{
			Bytes: firstPageWords[len(firstPageWords)-1].ID,
			Valid: true,
		},
	}
	lastPageWords, err := testStore.ListWords(context.Background(), lastPageParams)
	expectedNumLastPageWords := len(words) - lastSeenWordIdx
	require.NoError(t, err)
	require.NotEmpty(t, lastPageWords)
	require.Len(t, lastPageWords, expectedNumLastPageWords)
	for _, word := range lastPageWords {
		require.Equal(t, words[len(words)-lastSeenWordIdx-1], word.Word)
		lastSeenWordIdx += 1
	}
}

func TestGetRandomWordsForServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	testCases := []struct {
		name                    string
		words                   []string
		numWordsToFetch         int32
		expectedNumFetchedWords int
	}{
		{
			name:                    "exactly n words in db",
			words:                   []string{gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun()},
			numWordsToFetch:         5,
			expectedNumFetchedWords: 5,
		},
		{
			name:                    "more than n words in db",
			words:                   []string{gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun()},
			numWordsToFetch:         3,
			expectedNumFetchedWords: 3,
		},
		{
			name:                    "less than n words in db",
			words:                   []string{gofakeit.Noun(), gofakeit.Noun()},
			numWordsToFetch:         5,
			expectedNumFetchedWords: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := createTestServer(t, uuid.New())
			require.NotEmpty(t, server)

			addWordsParams := AddWordsToServerParams{
				ServerID: server.ID,
				Words:    tc.words,
			}
			addWordsRes, err := testStore.AddWordsToServer(context.Background(), addWordsParams)
			require.NoError(t, err)
			require.NotEmpty(t, addWordsRes)

			getWordsParams := GetRandomWordsForServerParams{
				ServerID: server.ID,
				N:        tc.numWordsToFetch,
			}
			fetchedWords, err := testStore.GetRandomWordsForServer(context.Background(), getWordsParams)
			require.NoError(t, err)
			require.NotEmpty(t, fetchedWords)

			require.Len(t, fetchedWords, tc.expectedNumFetchedWords)

			seen := make(map[string]bool)
			for _, word := range fetchedWords {
				require.False(t, seen[word], "duplicate word found: %s", word)
				seen[word] = true
			}
		})
	}
}

func TestRemoveWordsFromServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	words := []string{gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun()}
	addWordsParams := AddWordsToServerParams{
		ServerID: server.ID,
		Words:    words,
	}

	resAddWords, err := testStore.AddWordsToServer(context.Background(), addWordsParams)
	require.NoError(t, err)
	require.NotEmpty(t, resAddWords)
	require.Equal(t, int64(4), resAddWords.RowsAffected())

	rand.Shuffle(len(words), func(i, j int) {
		words[i], words[j] = words[j], words[i]
	})
	wordsToRemove := append([]string{}, words[:2]...)
	wordsToRemove = append(wordsToRemove, "word-that-wasn't-there")
	removeWordsParams := RemoveWordsFromServerParams{
		ServerID: server.ID,
		Words:    wordsToRemove,
	}
	resRemoveWords, err := testStore.RemoveWordsFromServer(context.Background(), removeWordsParams)
	require.NoError(t, err)
	require.NotEmpty(t, resRemoveWords)
	require.Equal(t, int64(2), resRemoveWords.RowsAffected())

	resListWords, err := testStore.ListWords(context.Background(), ListWordsParams{
		ServerID: server.ID,
		PageSize: 5,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resListWords)
	require.Len(t, resListWords, 2)
}

func TestRemoveAllWordsFromServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db test in short mode")
	}

	server := createTestServer(t, uuid.New())
	require.NotEmpty(t, server)

	words := []string{gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun(), gofakeit.Noun()}
	addWordsParams := AddWordsToServerParams{
		ServerID: server.ID,
		Words:    words,
	}

	resAddWords, err := testStore.AddWordsToServer(context.Background(), addWordsParams)
	require.NoError(t, err)
	require.NotEmpty(t, resAddWords)
	require.Equal(t, int64(4), resAddWords.RowsAffected())

	resRemoveAllWords, err := testStore.RemoveAllWordsFromServer(context.Background(), server.ID)
	require.NoError(t, err)
	require.NotEmpty(t, resRemoveAllWords)

	resListWords, err := testStore.ListWords(context.Background(), ListWordsParams{
		ServerID: server.ID,
		PageSize: 5,
	})
	require.NoError(t, err)
	require.Empty(t, resListWords)
	require.Len(t, resListWords, 0)
}
