package gameserver

import (
	"context"
	"errors"
	"testing"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	mockgameflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"

	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	mockwordstore "github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetWordChoices(t *testing.T) {
	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore, cache *mockcache.MockCache, wordStore *mockwordstore.MockStore)
		expectErr     bool
		expectedWords []string
	}{
		{
			name: "success",
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache, wordStore *mockwordstore.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						NumDrawingOptions: 2,
					}, nil)

				wordStore.EXPECT().
					GetRandomWords(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return([]wordstore.Word{
						{Word: "car"}, {Word: "bike"},
					}, nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			expectErr:     false,
			expectedWords: []string{"car", "bike"},
		},
		{
			name: "could not get server by id",
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache, wordStore *mockwordstore.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, errors.New("could not get server by id"))
			},
			expectErr:     true,
			expectedWords: nil,
		},
		{
			name: "could not get words",
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache, wordStore *mockwordstore.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						NumDrawingOptions: 2,
					}, nil)

				wordStore.EXPECT().
					GetRandomWords(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return([]wordstore.Word{}, errors.New("could not get words"))
			},
			expectErr:     true,
			expectedWords: nil,
		},
		{
			name: "could not cache words",
			buildStubs: func(store *mockdb.MockStore, cache *mockcache.MockCache, wordStore *mockwordstore.MockStore) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						NumDrawingOptions: 2,
					}, nil)

				wordStore.EXPECT().
					GetRandomWords(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return([]wordstore.Word{
						{Word: "car"}, {Word: "bike"},
					}, nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("could not cache words"))
			},
			expectErr:     true,
			expectedWords: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, gameServer := createTestGameServer(t)
			require.NotEmpty(t, gameServer)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			cache := mockcache.NewMockCache(ctrl)
			wordStore := mockwordstore.NewMockStore(ctrl)

			tc.buildStubs(store, cache, wordStore)

			gameServer.Config = Config{
				Store:     store,
				Cache:     cache,
				WordStore: wordStore,
				GfClient:  gameServer.GfClient,
				Env:       gameServer.Env,
				Bus:       gameServer.Bus,
			}

			words, err := gameServer.getWordChoices()

			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expectedWords, words)
		})
	}
}

func TestChooseWord(t *testing.T) {
	testCases := []struct {
		name             string
		buildStubs       func(cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient)
		selectedWord     string
		expectErr        bool
		knownExpectedErr error
	}{
		{
			name: "success",
			buildStubs: func(cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *[]string) error {
						*dest = []string{"car", "bike"}
						return nil
					})

				gfClient.EXPECT().
					SelectWord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq("car")).
					Times(1).
					Return(nil)
			},
			selectedWord:     "car",
			expectErr:        false,
			knownExpectedErr: nil,
		},
		{
			name: "no cached words found",
			buildStubs: func(cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *[]string) error {
						*dest = []string{}
						return commonCache.ErrKeyNotFound
					})
			},
			selectedWord:     "car",
			expectErr:        true,
			knownExpectedErr: ErrNoCachedWords,
		},
		{
			name: "error getting words from cache",
			buildStubs: func(cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *[]string) error {
						*dest = []string{}
						return errors.New("could not get words")
					})
			},
			selectedWord:     "car",
			expectErr:        true,
			knownExpectedErr: nil,
		},
		{
			name: "selected word not in choices",
			buildStubs: func(cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *[]string) error {
						*dest = []string{"car", "bike"}
						return nil
					})
			},
			selectedWord:     "boat",
			expectErr:        true,
			knownExpectedErr: ErrWordNotInChoices,
		},
		{
			name: "could not select word",
			buildStubs: func(cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *[]string) error {
						*dest = []string{"car", "bike"}
						return nil
					})

				gfClient.EXPECT().
					SelectWord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq("car")).
					Times(1).
					Return(errors.New("could not select word"))
			},
			selectedWord:     "car",
			expectErr:        true,
			knownExpectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mocks, gs := createInitializedTestGameServer(t)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tc.buildStubs(mocks.Cache, mocks.GfClient)

			err := gs.chooseWord(tc.selectedWord)

			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tc.knownExpectedErr != nil {
				require.Equal(t, tc.knownExpectedErr, err)
			}
		})
	}
}
