package wordstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetRandomWords(t *testing.T) {
	testCases := []struct {
		name          string
		buildStubs    func(dbStore *mockdb.MockStore)
		numWords      int
		checkResponse func(t *testing.T, words []Word, err error)
	}{
		{
			name: "get random default words",
			buildStubs: func(dbStore *mockdb.MockStore) {
				dbStore.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						NumCustomWords: 0,
					}, nil)
			},
			numWords: 20,
			checkResponse: func(t *testing.T, words []Word, err error) {
				require.NoError(t, err)
				require.Len(t, words, 20)
			},
		},
		{
			name: "get random server words",
			buildStubs: func(dbStore *mockdb.MockStore) {
				dbStore.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						NumCustomWords: 3,
					}, nil)

				dbStore.EXPECT().
					GetRandomWordsForServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetRandomWordsForServerRow{
						{Word: "word-1", AddedAt: time.Now().UTC()},
						{Word: "word-2", AddedAt: time.Now().UTC()},
						{Word: "word-3", AddedAt: time.Now().UTC()},
					}, nil)
			},
			numWords: 5,
			checkResponse: func(t *testing.T, words []Word, err error) {
				require.NoError(t, err)
				require.Len(t, words, 3)
			},
		},
		{
			name: "could not get server from db",
			buildStubs: func(dbStore *mockdb.MockStore) {
				dbStore.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, db.RecordNotFoundError)
			},
			numWords: 2,
			checkResponse: func(t *testing.T, words []Word, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, ErrServerNotfound.Error())
				require.Empty(t, words)
			},
		},
		{
			name: "unexpected db error",
			buildStubs: func(dbStore *mockdb.MockStore) {
				dbStore.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, errors.New("some db error"))
			},
			numWords: 2,
			checkResponse: func(t *testing.T, words []Word, err error) {
				require.Error(t, err)
				require.Empty(t, words)
			},
		},
		{
			name: "could not get random words from db",
			buildStubs: func(dbStore *mockdb.MockStore) {
				dbStore.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						NumCustomWords: 3,
					}, nil)

				dbStore.EXPECT().
					GetRandomWordsForServer(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.GetRandomWordsForServerRow{}, errors.New("some db error"))
			},
			numWords: 2,
			checkResponse: func(t *testing.T, words []Word, err error) {
				require.Error(t, err)
				require.Empty(t, words)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			dbStore := mockdb.NewMockStore(ctrl)

			tc.buildStubs(dbStore)

			store, err := NewDefaultStore()
			require.NoError(t, err)

			randomWords, err := store.GetRandomWords(context.Background(), dbStore, uuid.New(), tc.numWords)
			tc.checkResponse(t, randomWords, err)
		})
	}
}
