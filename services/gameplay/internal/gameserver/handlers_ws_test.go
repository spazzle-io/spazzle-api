package gameserver

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	mockgameflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	mockwordstore "github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandleGetWordChoices(t *testing.T) {
	currentArtist := createTestClient(t, false)

	testCases := []struct {
		name          string
		msg           func() []byte
		client        *Client
		buildStubs    func(gameServer *GameServer, store *mockdb.MockStore, wordStore *mockwordstore.MockStore, cache *mockcache.MockCache)
		errorSent     bool
		directMsgSent bool
	}{
		{
			name: "success",
			msg: func() []byte {
				wsMsg := WsMessage{
					Type: gameevents.TypeGetWordChoices,
				}

				msg, err := json.Marshal(wsMsg)
				require.NoError(t, err)
				require.NotEmpty(t, msg)

				return msg
			},
			client: currentArtist,
			buildStubs: func(gameServer *GameServer, store *mockdb.MockStore, wordStore *mockwordstore.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						NumDrawingOptions: 2,
					}, nil)

				wordStore.EXPECT().
					GetRandomWords(gomock.Any(), gomock.Eq(store), gomock.Any(), gomock.Eq(2)).
					Times(1).
					Return([]wordstore.Word{
						{Word: "car"}, {Word: "bicycle"},
					}, nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			errorSent:     false,
			directMsgSent: true,
		},
		{
			name: "invalid ws message",
			msg: func() []byte {
				return []byte("invalid ws message")
			},
			client: currentArtist,
			buildStubs: func(gameServer *GameServer, store *mockdb.MockStore, wordStore *mockwordstore.MockStore, cache *mockcache.MockCache) {
			},
			errorSent:     false,
			directMsgSent: false,
		},
		{
			name: "client not current artist",
			msg: func() []byte {
				wsMsg := WsMessage{
					Type: gameevents.TypeGetWordChoices,
				}

				msg, err := json.Marshal(wsMsg)
				require.NoError(t, err)
				require.NotEmpty(t, msg)

				return msg
			},
			client: createTestClient(t, false),
			buildStubs: func(gameServer *GameServer, store *mockdb.MockStore, wordStore *mockwordstore.MockStore, cache *mockcache.MockCache) {
			},
			errorSent:     false,
			directMsgSent: false,
		},
		{
			name: "could not get word choices",
			msg: func() []byte {
				wsMsg := WsMessage{
					Type: gameevents.TypeGetWordChoices,
				}

				msg, err := json.Marshal(wsMsg)
				require.NoError(t, err)
				require.NotEmpty(t, msg)

				return msg
			},
			client: currentArtist,
			buildStubs: func(gameServer *GameServer, store *mockdb.MockStore, wordStore *mockwordstore.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{}, errors.New("could not get server"))
			},
			errorSent:     true,
			directMsgSent: false,
		},
		{
			name: "could not send word choices to client",
			msg: func() []byte {
				wsMsg := WsMessage{
					Type: gameevents.TypeGetWordChoices,
				}

				msg, err := json.Marshal(wsMsg)
				require.NoError(t, err)
				require.NotEmpty(t, msg)

				return msg
			},
			client: currentArtist,
			buildStubs: func(gameServer *GameServer, store *mockdb.MockStore, wordStore *mockwordstore.MockStore, cache *mockcache.MockCache) {
				store.EXPECT().
					GetServerById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Server{
						NumDrawingOptions: 2,
					}, nil)

				wordStore.EXPECT().
					GetRandomWords(gomock.Any(), gomock.Eq(store), gomock.Any(), gomock.Eq(2)).
					Times(1).
					Return([]wordstore.Word{
						{Word: "car"}, {Word: "bicycle"},
					}, nil)

				cache.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				gameServer.isClosed.CompareAndSwap(false, true)
			},
			errorSent:     false,
			directMsgSent: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, gameServer := createTestGameServer(t)
			require.NotEmpty(t, gameServer)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			wordStore := mockwordstore.NewMockStore(ctrl)
			cache := mockcache.NewMockCache(ctrl)

			tc.buildStubs(gameServer, store, wordStore, cache)

			gameServer.Config = Config{
				Store:     store,
				Cache:     cache,
				WordStore: wordStore,
				Env:       gameServer.Env,
				Bus:       gameServer.Bus,
				GfClient:  gameServer.GfClient,
			}

			directMsgSentCh := make(chan string, 1)

			go func() {
				select {
				case msg := <-gameServer.directMsg:
					directMsgSentCh <- msg.Msg.Type
				case <-time.After(1 * time.Second):
					directMsgSentCh <- "failed to send directMsg"
				}
			}()

			gameServer.setCurrentArtist(currentArtist.userID)
			gameServer.handleClientWsMessage(tc.client, tc.msg())

			directMsgSent := <-directMsgSentCh

			if tc.directMsgSent {
				require.Equal(t, gameevents.TypeGetWordChoices, directMsgSent)
			}

			if tc.errorSent {
				require.Equal(t, TypeClientError, directMsgSent)
			}
		})
	}
}

func TestHandleSelectWord(t *testing.T) {
	currentArtist := createTestClient(t, false)
	workflowSelectionID := uuid.New()

	testCases := []struct {
		name       string
		msg        func() []byte
		client     *Client
		buildStubs func(gameServer *GameServer, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient)
		errorSent  bool
	}{
		{
			name: "success",
			msg: func() []byte {
				payload, err := json.Marshal(gameevents.SelectWordPayload{
					Word: "Gandalf",
				})
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				wsMsg := WsMessage{
					Type:    gameevents.TypeSelectWord,
					Payload: payload,
				}

				msg, err := json.Marshal(wsMsg)
				require.NoError(t, err)
				require.NotEmpty(t, msg)

				return msg
			},
			client: currentArtist,
			buildStubs: func(gameServer *GameServer, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *[]string) error {
						*dest = []string{"Saruman", "Gandalf", "Radagast"}
						return nil
					})

				gfClient.EXPECT().
					SelectWord(gomock.Any(), gomock.Eq("Gandalf"), gomock.Eq(workflowSelectionID)).
					Times(1).
					Return(nil)
			},
			errorSent: false,
		},
		{
			name: "invalid ws message",
			msg: func() []byte {
				return []byte("invalid ws message")
			},
			client:     currentArtist,
			buildStubs: func(gameServer *GameServer, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient) {},
			errorSent:  false,
		},
		{
			name: "client not current artist",
			msg: func() []byte {
				payload, err := json.Marshal(gameevents.SelectWordPayload{
					Word: "Gandalf",
				})
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				wsMsg := WsMessage{
					Type:    gameevents.TypeSelectWord,
					Payload: payload,
				}

				msg, err := json.Marshal(wsMsg)
				require.NoError(t, err)
				require.NotEmpty(t, msg)

				return msg
			},
			client:     createTestClient(t, false),
			buildStubs: func(gameServer *GameServer, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient) {},
			errorSent:  false,
		},
		{
			name: "no cached words",
			msg: func() []byte {
				payload, err := json.Marshal(gameevents.SelectWordPayload{
					Word: "Gandalf",
				})
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				wsMsg := WsMessage{
					Type:    gameevents.TypeSelectWord,
					Payload: payload,
				}

				msg, err := json.Marshal(wsMsg)
				require.NoError(t, err)
				require.NotEmpty(t, msg)

				return msg
			},
			client: currentArtist,
			buildStubs: func(gameServer *GameServer, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *[]string) error {
						*dest = []string{}
						return ErrNoCachedWords
					})
			},
			errorSent: true,
		},
		{
			name: "word not in choices",
			msg: func() []byte {
				payload, err := json.Marshal(gameevents.SelectWordPayload{
					Word: "Gandalf",
				})
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				wsMsg := WsMessage{
					Type:    gameevents.TypeSelectWord,
					Payload: payload,
				}

				msg, err := json.Marshal(wsMsg)
				require.NoError(t, err)
				require.NotEmpty(t, msg)

				return msg
			},
			client: currentArtist,
			buildStubs: func(gameServer *GameServer, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *[]string) error {
						*dest = []string{}
						return ErrWordNotInChoices
					})
			},
			errorSent: true,
		},
		{
			name: "failed to select word",
			msg: func() []byte {
				payload, err := json.Marshal(gameevents.SelectWordPayload{
					Word: "Gandalf",
				})
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				wsMsg := WsMessage{
					Type:    gameevents.TypeSelectWord,
					Payload: payload,
				}

				msg, err := json.Marshal(wsMsg)
				require.NoError(t, err)
				require.NotEmpty(t, msg)

				return msg
			},
			client: currentArtist,
			buildStubs: func(gameServer *GameServer, cache *mockcache.MockCache, gfClient *mockgameflowclient.MockClient) {
				cache.EXPECT().
					Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx context.Context, key string, dest *[]string) error {
						*dest = []string{}
						return errors.New("failed to fetch words")
					})
			},
			errorSent: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, gameServer := createTestGameServer(t)
			require.NotEmpty(t, gameServer)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cache := mockcache.NewMockCache(ctrl)
			gfClient := mockgameflowclient.NewMockClient(ctrl)

			tc.buildStubs(gameServer, cache, gfClient)

			gameServer.Config = Config{
				Cache:     cache,
				GfClient:  gfClient,
				WordStore: gameServer.WordStore,
				Store:     gameServer.Store,
				Env:       gameServer.Env,
				Bus:       gameServer.Bus,
			}

			directMsgSentCh := make(chan string, 1)

			go func() {
				select {
				case msg := <-gameServer.directMsg:
					directMsgSentCh <- msg.Msg.Type
				case <-time.After(1 * time.Second):
					directMsgSentCh <- "failed to send directMsg"
				}
			}()

			gameServer.setCurrentArtist(currentArtist.userID)
			gameServer.setWorkflowSelectionID(workflowSelectionID)
			gameServer.handleClientWsMessage(tc.client, tc.msg())

			directMsgSent := <-directMsgSentCh

			if tc.errorSent {
				require.Equal(t, TypeClientError, directMsgSent)
			}
		})
	}
}

func TestIsCurrentArtist(t *testing.T) {
	validCurrentArtist := createTestClient(t, false)

	testCases := []struct {
		name     string
		c        func() *Client
		expected bool
	}{
		{
			name: "is current artist",
			c: func() *Client {
				return validCurrentArtist
			},
			expected: true,
		},
		{
			name: "not current artist - spectating",
			c: func() *Client {
				currentArtist := *validCurrentArtist
				currentArtist.isSpectating = true

				return &currentArtist
			},
			expected: false,
		},
		{
			name: "not current artist - different client",
			c: func() *Client {
				return createTestClient(t, false)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, gameServer := createTestGameServer(t)
			require.NotEmpty(t, gameServer)

			gameServer.setCurrentArtist(validCurrentArtist.userID)
			isCurrentArtist := gameServer.isCurrentArtist(tc.c())

			require.Equal(t, tc.expected, isCurrentArtist)
		})
	}
}
