package gameserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"

	"github.com/google/uuid"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newClientWsMsg(t *testing.T, msgType string, payload any) []byte {
	t.Helper()

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	msg, err := json.Marshal(WsMessage{
		Type:    msgType,
		Payload: raw,
	})

	require.NoError(t, err)
	return msg
}

func registerClientOnServer(t *testing.T, gs *GameServer, c *Client) {
	t.Helper()

	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.clients[c.userID] == nil {
		gs.clients[c.userID] = map[uuid.UUID]*Client{}
	}
	gs.clients[c.userID][c.connID] = c
}

func makeActivePlayer(t *testing.T, gs *GameServer, userID uuid.UUID) *Client {
	c := newStubClient(t, gs, userID, Player)
	registerClientOnServer(t, gs, c)

	gs.mu.Lock()
	gs.activePlayers[userID] = true
	gs.mu.Unlock()

	return c
}

func makeArtistClient(t *testing.T, gs *GameServer) *Client {
	artistID := gs.getCurrentArtist()
	c := newStubClient(t, gs, artistID, Player)
	registerClientOnServer(t, gs, c)

	return c
}

func requireClientReceivedMsg(t *testing.T, c *Client) *OutgoingMessage {
	t.Helper()

	select {
	case outMsg := <-c.send:
		require.NotNil(t, outMsg)
		return outMsg
	case <-time.After(broadcastTimeout):
		t.Fatal("expected message on client send channel")
		return nil
	}
}

func requireClientNoMsg(t *testing.T, c *Client) {
	t.Helper()

	select {
	case outMsg := <-c.send:
		t.Fatalf("unexpected message on client send channel: %+v", outMsg)
	case <-time.After(200 * time.Millisecond):
		// ok. nothing received.
	}
}

func TestHandleClientWsMessage_IgnoresNonJoinWhenGameInactive(t *testing.T) {
	_, gs := createTestGameServer(t)
	gs.isGameActive.Store(false)

	client := newStubClient(t, gs, uuid.New(), Player)
	registerClientOnServer(t, gs, client)

	msg := newClientWsMsg(t, gameevents.TypeGuessWord, gameevents.GuessWordPayload{Guess: "test"})

	gs.handleClientWsMessage(client, msg)

	requireClientNoMsg(t, client)
}

func TestHandleClientWsMessage_InvalidJSON(t *testing.T) {
	_, gs := createInitializedTestGameServer(t)

	client := newStubClient(t, gs, uuid.New(), Player)
	registerClientOnServer(t, gs, client)

	gs.handleClientWsMessage(client, []byte(`not json`))

	requireClientNoMsg(t, client)
}

func TestHandleJoinGame(t *testing.T) {
	t.Run("successful registration", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)
		client := makeActivePlayer(t, gs, uuid.New())

		mocks.Cache.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, key string, dest *gamecache.JoinCodeEntry) error {
				*dest = gamecache.JoinCodeEntry{
					UserID:   client.userID,
					ServerID: gs.serverID,
					GameID:   gs.GetGameID(),
					Role:     string(client.Role()),
				}
				return nil
			})

		mocks.Cache.EXPECT().
			Del(gomock.Any(), gomock.Any()).
			Times(1).
			Return(nil)

		registerCh := make(chan bool, 1)
		go func() {
			select {
			case <-gs.register:
				registerCh <- true
			case <-time.After(broadcastTimeout):
				registerCh <- false
			}
		}()

		msg := newClientWsMsg(t, gameevents.TypeJoinGame, gameevents.JoinGamePayload{JoinCode: "valid code"})
		gs.handleClientWsMessage(client, msg)

		requireClientNoMsg(t, client)
		require.True(t, <-registerCh)
		require.Equal(t, Player, client.Role())
	})

	t.Run("invalid payload sends error", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)
		client := makeActivePlayer(t, gs, uuid.New())

		registerCh := make(chan bool, 1)
		go func() {
			select {
			case <-gs.register:
				registerCh <- true
			case <-time.After(broadcastTimeout):
				registerCh <- false
			}
		}()

		msg := newClientWsMsg(t, gameevents.TypeJoinGame, `{invalid`)
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
		require.False(t, <-registerCh)
		require.Equal(t, Player, client.Role())
	})

	t.Run("joining a closed server sends error", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)
		gs.isClosed.Store(true)

		client := makeActivePlayer(t, gs, uuid.New())

		registerCh := make(chan bool, 1)
		go func() {
			select {
			case <-gs.register:
				registerCh <- true
			case <-time.After(broadcastTimeout):
				registerCh <- false
			}
		}()

		msg := newClientWsMsg(t, gameevents.TypeJoinGame, gameevents.JoinGamePayload{JoinCode: "valid code"})
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
		require.False(t, <-registerCh)
		require.Equal(t, Player, client.Role())
	})

	t.Run("joining an inactive game sends error", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)
		gs.isGameActive.Store(false)

		client := makeActivePlayer(t, gs, uuid.New())

		registerCh := make(chan bool, 1)
		go func() {
			select {
			case <-gs.register:
				registerCh <- true
			case <-time.After(broadcastTimeout):
				registerCh <- false
			}
		}()

		payload, err := json.Marshal(gameevents.JoinGamePayload{JoinCode: "valid code"})
		require.NoError(t, err)
		require.NotEmpty(t, payload)

		gs.handleJoinGame(client, &WsMessage{
			Type:    gameevents.TypeJoinGame,
			Payload: payload,
		})

		requireClientReceivedMsg(t, client)
		require.False(t, <-registerCh)
		require.Equal(t, Player, client.Role())
	})

	t.Run("could not validate join code", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)
		client := makeActivePlayer(t, gs, uuid.New())

		mocks.Cache.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, key string, dest *gamecache.JoinCodeEntry) error {
				*dest = gamecache.JoinCodeEntry{}
				return errors.New("could not validate join code")
			})

		registerCh := make(chan bool, 1)
		go func() {
			select {
			case <-gs.register:
				registerCh <- true
			case <-time.After(broadcastTimeout):
				registerCh <- false
			}
		}()

		msg := newClientWsMsg(t, gameevents.TypeJoinGame, gameevents.JoinGamePayload{JoinCode: "valid code"})
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
		require.False(t, <-registerCh)
		require.Equal(t, Player, client.Role())
	})

	t.Run("join code user id does not match calling user id", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)
		client := makeActivePlayer(t, gs, uuid.New())

		mocks.Cache.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, key string, dest *gamecache.JoinCodeEntry) error {
				*dest = gamecache.JoinCodeEntry{
					UserID:   uuid.New(),
					ServerID: gs.serverID,
					GameID:   gs.GetGameID(),
					Role:     string(client.Role()),
				}
				return nil
			})

		registerCh := make(chan bool, 1)
		go func() {
			select {
			case <-gs.register:
				registerCh <- true
			case <-time.After(broadcastTimeout):
				registerCh <- false
			}
		}()

		msg := newClientWsMsg(t, gameevents.TypeJoinGame, gameevents.JoinGamePayload{JoinCode: "valid code"})
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
		require.False(t, <-registerCh)
		require.Equal(t, Player, client.Role())
	})

	t.Run("join code role could not be parsed", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)
		client := makeActivePlayer(t, gs, uuid.New())

		mocks.Cache.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, key string, dest *gamecache.JoinCodeEntry) error {
				*dest = gamecache.JoinCodeEntry{
					UserID:   client.userID,
					ServerID: gs.serverID,
					GameID:   gs.GetGameID(),
					Role:     "invalid role",
				}
				return nil
			})

		registerCh := make(chan bool, 1)
		go func() {
			select {
			case <-gs.register:
				registerCh <- true
			case <-time.After(broadcastTimeout):
				registerCh <- false
			}
		}()

		msg := newClientWsMsg(t, gameevents.TypeJoinGame, gameevents.JoinGamePayload{JoinCode: "valid code"})
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
		require.False(t, <-registerCh)
		require.Equal(t, Player, client.Role())
	})

	t.Run("could not invalidate join code", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)
		client := makeActivePlayer(t, gs, uuid.New())

		mocks.Cache.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, key string, dest *gamecache.JoinCodeEntry) error {
				*dest = gamecache.JoinCodeEntry{
					UserID:   client.userID,
					ServerID: gs.serverID,
					GameID:   gs.GetGameID(),
					Role:     string(client.Role()),
				}
				return nil
			})

		mocks.Cache.EXPECT().
			Del(gomock.Any(), gomock.Any()).
			Times(1).
			Return(errors.New("could not invalidate join code"))

		registerCh := make(chan bool, 1)
		go func() {
			select {
			case <-gs.register:
				registerCh <- true
			case <-time.After(broadcastTimeout):
				registerCh <- false
			}
		}()

		msg := newClientWsMsg(t, gameevents.TypeJoinGame, gameevents.JoinGamePayload{JoinCode: "valid code"})
		gs.handleClientWsMessage(client, msg)

		requireClientNoMsg(t, client)
		require.True(t, <-registerCh)
		require.Equal(t, Player, client.Role())
	})

	t.Run("join game with different role", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)
		client := makeActivePlayer(t, gs, uuid.New())

		mocks.Cache.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, key string, dest *gamecache.JoinCodeEntry) error {
				*dest = gamecache.JoinCodeEntry{
					UserID:   client.userID,
					ServerID: gs.serverID,
					GameID:   gs.GetGameID(),
					Role:     string(Moderator),
				}
				return nil
			})

		mocks.Cache.EXPECT().
			Del(gomock.Any(), gomock.Any()).
			Times(1).
			Return(nil)

		registerCh := make(chan bool, 1)
		go func() {
			select {
			case <-gs.register:
				registerCh <- true
			case <-time.After(broadcastTimeout):
				registerCh <- false
			}
		}()

		msg := newClientWsMsg(t, gameevents.TypeJoinGame, gameevents.JoinGamePayload{JoinCode: "valid code"})
		gs.handleClientWsMessage(client, msg)

		requireClientNoMsg(t, client)
		require.True(t, <-registerCh)
		require.Equal(t, Moderator, client.Role())
	})
}

func TestHandleGuessWord(t *testing.T) {
	t.Run("correct guess records and publishes", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		guesserID := uuid.New()
		client := makeActivePlayer(t, gs, guesserID)

		gs.mu.Lock()
		gs.currentWord = "banana"
		gs.mu.Unlock()

		mocks.GfClient.EXPECT().
			RecordCorrectGuesses(
				gomock.Eq(gs.serverID),
				gomock.Eq(gs.GetGameID()),
				gomock.Eq(gs.getCurrentRound()),
				gomock.Any(),
			).Times(1)

		mocks.Session.EXPECT().
			Publish(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ eventbus.StreamType, pm eventbus.PublishMessage) (string, error) {
				require.Equal(t, gameevents.TypeWordGuessed, pm.Type)

				var p gameevents.WordGuessedPayload
				require.NoError(t, json.Unmarshal(pm.Payload, &p))
				require.True(t, p.IsCorrect)
				require.Equal(t, guesserID, p.PlayerID)
				require.Empty(t, p.Guess)
				return "msg-id", nil
			})

		msg := newClientWsMsg(t, gameevents.TypeGuessWord, gameevents.GuessWordPayload{Guess: "banana"})
		gs.handleClientWsMessage(client, msg)

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.True(t, gs.correctGuessers[guesserID])
	})

	t.Run("incorrect guess publishes with guess text visible", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		guesserID := uuid.New()
		client := makeActivePlayer(t, gs, guesserID)

		gs.mu.Lock()
		gs.currentWord = "banana"
		gs.mu.Unlock()

		mocks.Session.EXPECT().
			Publish(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ eventbus.StreamType, pm eventbus.PublishMessage) (string, error) {
				var p gameevents.WordGuessedPayload
				require.NoError(t, json.Unmarshal(pm.Payload, &p))
				require.False(t, p.IsCorrect)
				require.Equal(t, "apple", p.Guess)
				return "msg-id", nil
			})

		msg := newClientWsMsg(t, gameevents.TypeGuessWord, gameevents.GuessWordPayload{Guess: "apple"})
		gs.handleClientWsMessage(client, msg)

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.False(t, gs.correctGuessers[guesserID])
	})

	t.Run("artist cannot guess", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		artistID := gs.getCurrentArtist()
		_ = makeActivePlayer(t, gs, artistID)
		client := makeArtistClient(t, gs)

		msg := newClientWsMsg(t, gameevents.TypeGuessWord, gameevents.GuessWordPayload{Guess: "test"})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("spectator cannot guess", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		spectatorID := uuid.New()
		client := newStubClient(t, gs, spectatorID, Spectator)
		registerClientOnServer(t, gs, client)

		gs.mu.Lock()
		gs.activePlayers[spectatorID] = true
		gs.mu.Unlock()

		msg := newClientWsMsg(t, gameevents.TypeGuessWord, gameevents.GuessWordPayload{Guess: "test"})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("already correct guesser can still chat", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		guesserID := uuid.New()
		client := makeActivePlayer(t, gs, guesserID)

		gs.mu.Lock()
		gs.currentWord = "banana"
		gs.correctGuessers[guesserID] = true
		gs.mu.Unlock()

		mocks.Session.EXPECT().
			Publish(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ eventbus.StreamType, pm eventbus.PublishMessage) (string, error) {
				var p gameevents.WordGuessedPayload
				require.NoError(t, json.Unmarshal(pm.Payload, &p))
				require.False(t, p.IsCorrect)
				require.Equal(t, "apple", p.Guess)
				return "msg-id", nil
			})

		msg := newClientWsMsg(t, gameevents.TypeGuessWord, gameevents.GuessWordPayload{Guess: "apple"})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("already correct guesser cannot score again", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		guesserID := uuid.New()
		client := makeActivePlayer(t, gs, guesserID)

		gs.mu.Lock()
		gs.currentWord = "banana"
		gs.correctGuessers[guesserID] = true
		gs.mu.Unlock()

		msg := newClientWsMsg(t, gameevents.TypeGuessWord, gameevents.GuessWordPayload{Guess: "banana"})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("inactive player cannot guess", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		outsiderID := uuid.New()
		client := newStubClient(t, gs, outsiderID, Player)
		registerClientOnServer(t, gs, client)

		msg := newClientWsMsg(t, gameevents.TypeGuessWord, gameevents.GuessWordPayload{Guess: "test"})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("whitespace-only guess is ignored", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		guesserID := uuid.New()
		client := makeActivePlayer(t, gs, guesserID)

		msg := newClientWsMsg(t, gameevents.TypeGuessWord, gameevents.GuessWordPayload{Guess: "   "})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("publish failure sends error to client", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		guesserID := uuid.New()
		client := makeActivePlayer(t, gs, guesserID)

		gs.mu.Lock()
		gs.currentWord = "banana"
		gs.mu.Unlock()

		mocks.Session.EXPECT().
			Publish(gomock.Any(), gomock.Any(), gomock.Any()).
			Return("", fmt.Errorf("bus unavailable"))

		msg := newClientWsMsg(t, gameevents.TypeGuessWord, gameevents.GuessWordPayload{Guess: "apple"})
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
	})
}

func TestHandleSelectWord(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		artistID := gs.getCurrentArtist()
		_ = makeActivePlayer(t, gs, artistID)
		client := makeArtistClient(t, gs)

		mocks.Cache.EXPECT().
			Get(gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx context.Context, key string, dest *[]string) error {
				*dest = append(*dest, "banana")
				return nil
			})

		mocks.GfClient.EXPECT().
			SelectWord(gomock.Eq(gs.serverID), gomock.Eq(gs.gameID), gomock.Eq(gs.getCurrentRound()), gomock.Eq("banana")).
			Times(1).
			Return(nil)

		msg := newClientWsMsg(t, gameevents.TypeSelectWord, gameevents.SelectWordPayload{Word: "banana"})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("non-artist cannot select word", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		nonArtist := uuid.New()
		client := newStubClient(t, gs, nonArtist, Player)
		registerClientOnServer(t, gs, client)

		msg := newClientWsMsg(t, gameevents.TypeSelectWord, gameevents.SelectWordPayload{Word: "test"})
		gs.handleClientWsMessage(client, msg)

		requireClientNoMsg(t, client)
	})

	t.Run("empty word sends error", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)
		client := makeArtistClient(t, gs)

		msg := newClientWsMsg(t, gameevents.TypeSelectWord, gameevents.SelectWordPayload{Word: "  "})
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
	})

	t.Run("invalid payload sends error", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)
		client := makeArtistClient(t, gs)

		msg := newClientWsMsg(t, gameevents.TypeSelectWord, `{invalid`)
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
	})

	t.Run("chooseWord errors", func(t *testing.T) {
		testCases := []struct {
			name       string
			buildStubs func(mocks *MockConfig, gs *GameServer)
		}{
			{
				name: "no cached words",
				buildStubs: func(mocks *MockConfig, gs *GameServer) {
					mocks.Cache.EXPECT().
						Get(gomock.Any(), gomock.Any(), gomock.Any()).
						Return(ErrNoCachedWords)
				},
			},
			{
				name: "word not in choices",
				buildStubs: func(mocks *MockConfig, gs *GameServer) {
					mocks.Cache.EXPECT().
						Get(gomock.Any(), gomock.Any(), gomock.Any()).
						DoAndReturn(func(_ context.Context, _ string, dest *[]string) error {
							*dest = append(*dest, "orange")
							return nil
						})
				},
			},
			{
				name: "select word server error",
				buildStubs: func(mocks *MockConfig, gs *GameServer) {
					mocks.Cache.EXPECT().
						Get(gomock.Any(), gomock.Any(), gomock.Any()).
						DoAndReturn(func(_ context.Context, _ string, dest *[]string) error {
							*dest = append(*dest, "banana")
							return nil
						})

					mocks.GfClient.EXPECT().
						SelectWord(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return(fmt.Errorf("internal failure"))
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mocks, gs := createInitializedTestGameServer(t)
				client := makeArtistClient(t, gs)

				tc.buildStubs(mocks, gs)

				msg := newClientWsMsg(t, gameevents.TypeSelectWord, gameevents.SelectWordPayload{Word: "banana"})
				gs.handleClientWsMessage(client, msg)

				requireClientReceivedMsg(t, client)
			})
		}
	})
}

func TestHandleGetWordChoices(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)
		client := makeArtistClient(t, gs)

		gs.currentWord = ""

		mocks.Store.EXPECT().
			GetServerById(gomock.Any(), gomock.Eq(gs.serverID)).
			Times(1).
			Return(db.Server{
				NumDrawingOptions: 3,
			}, nil)

		mocks.WordStore.EXPECT().
			GetRandomWords(gomock.Any(), gomock.Any(), gomock.Eq(gs.serverID), gomock.Eq(3)).
			Times(1).
			Return([]wordstore.Word{
				{
					Word: "orange",
				},
				{
					Word: "banana",
				},
			}, nil)

		mocks.Cache.EXPECT().
			Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			Return(nil)

		msg := newClientWsMsg(t, gameevents.TypeGetWordChoices, struct{}{})
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
	})

	t.Run("non-artist cannot get word choices", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		nonArtist := uuid.New()
		client := newStubClient(t, gs, nonArtist, Player)
		registerClientOnServer(t, gs, client)

		msg := newClientWsMsg(t, gameevents.TypeGetWordChoices, struct{}{})
		gs.handleClientWsMessage(client, msg)

		requireClientNoMsg(t, client)
	})

	t.Run("skipped when word already selected", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)
		client := makeArtistClient(t, gs)

		require.NotEmpty(t, gs.getCurrentWord())

		msg := newClientWsMsg(t, gameevents.TypeGetWordChoices, struct{}{})
		gs.handleClientWsMessage(client, msg)

		requireClientNoMsg(t, client)
	})

	t.Run("error getting word choices", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)
		client := makeArtistClient(t, gs)

		gs.currentWord = ""

		mocks.Store.EXPECT().
			GetServerById(gomock.Any(), gomock.Eq(gs.serverID)).
			Times(1).
			Return(db.Server{}, errors.New("internal failure"))

		msg := newClientWsMsg(t, gameevents.TypeGetWordChoices, struct{}{})
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
	})
}

func TestHandleWarnPlayer(t *testing.T) {
	t.Run("publishes warning when caller has elevated permissions", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		callerID := uuid.New()
		client := newStubClient(t, gs, callerID, Player)
		registerClientOnServer(t, gs, client)

		targetID := uuid.New()

		mocks.Store.EXPECT().
			GetServerUserPermissions(gomock.Any(), db.GetServerUserPermissionsParams{
				ServerID: gs.serverID,
				UserID:   callerID,
			}).
			Return(db.GetServerUserPermissionsRow{HasElevatedPermissions: true}, nil)

		mocks.Session.EXPECT().
			Publish(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ eventbus.StreamType, pm eventbus.PublishMessage) (string, error) {
				require.Equal(t, gameevents.TypePlayerWarned, pm.Type)

				var p gameevents.PlayerWarnedPayload
				require.NoError(t, json.Unmarshal(pm.Payload, &p))
				require.Equal(t, targetID, p.PlayerID)
				return "msg-id", nil
			})

		msg := newClientWsMsg(t, gameevents.TypeWarnPlayer, gameevents.WarnPlayerPayload{PlayerID: targetID})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("silently ignored when caller lacks elevated permissions", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		callerID := uuid.New()
		client := newStubClient(t, gs, callerID, Player)
		registerClientOnServer(t, gs, client)

		mocks.Store.EXPECT().
			GetServerUserPermissions(gomock.Any(), db.GetServerUserPermissionsParams{
				ServerID: gs.serverID,
				UserID:   callerID,
			}).
			Return(db.GetServerUserPermissionsRow{HasElevatedPermissions: false}, nil)

		msg := newClientWsMsg(t, gameevents.TypeWarnPlayer, gameevents.WarnPlayerPayload{PlayerID: uuid.New()})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("permission check failure sends error to client", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		callerID := uuid.New()
		client := newStubClient(t, gs, callerID, Player)
		registerClientOnServer(t, gs, client)

		mocks.Store.EXPECT().
			GetServerUserPermissions(gomock.Any(), gomock.Any()).
			Return(db.GetServerUserPermissionsRow{}, fmt.Errorf("db error"))

		msg := newClientWsMsg(t, gameevents.TypeWarnPlayer, gameevents.WarnPlayerPayload{PlayerID: uuid.New()})
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
	})
}

func TestHandleReportPlayer(t *testing.T) {
	t.Run("active non-spectating player can report", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		reporterID := uuid.New()
		client := makeActivePlayer(t, gs, reporterID)

		reportedID := uuid.New()

		mocks.GfClient.EXPECT().
			ReportPlayer(gomock.Eq(gs.serverID), gomock.Eq(reporterID), gomock.Eq(reportedID)).
			Times(1)

		msg := newClientWsMsg(t, gameevents.TypeReportPlayer, gameevents.ReportPlayerPayload{ReportedID: reportedID})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("spectator cannot report", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		spectatorID := uuid.New()
		client := newStubClient(t, gs, spectatorID, Spectator)
		registerClientOnServer(t, gs, client)

		gs.mu.Lock()
		gs.activePlayers[spectatorID] = true
		gs.mu.Unlock()

		msg := newClientWsMsg(t, gameevents.TypeReportPlayer, gameevents.ReportPlayerPayload{ReportedID: uuid.New()})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("inactive player cannot report", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		outsiderID := uuid.New()
		client := newStubClient(t, gs, outsiderID, Player)
		registerClientOnServer(t, gs, client)

		msg := newClientWsMsg(t, gameevents.TypeReportPlayer, gameevents.ReportPlayerPayload{ReportedID: uuid.New()})
		gs.handleClientWsMessage(client, msg)
	})
}

func TestHandleClearPlayerReports(t *testing.T) {
	t.Run("clears reports when caller has elevated permissions", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		callerID := uuid.New()
		client := newStubClient(t, gs, callerID, Player)
		registerClientOnServer(t, gs, client)

		targetID := uuid.New()

		mocks.Store.EXPECT().
			GetServerUserPermissions(gomock.Any(), db.GetServerUserPermissionsParams{
				ServerID: gs.serverID,
				UserID:   callerID,
			}).
			Return(db.GetServerUserPermissionsRow{HasElevatedPermissions: true}, nil)

		mocks.GfClient.EXPECT().
			ClearPlayerReports(gomock.Eq(gs.serverID), gomock.Eq(targetID)).
			Times(1)

		msg := newClientWsMsg(t, gameevents.TypeClearPlayerReports, gameevents.ClearPlayerReportsPayload{PlayerID: targetID})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("silently ignored when caller lacks permissions", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		callerID := uuid.New()
		client := newStubClient(t, gs, callerID, Player)
		registerClientOnServer(t, gs, client)

		mocks.Store.EXPECT().
			GetServerUserPermissions(gomock.Any(), db.GetServerUserPermissionsParams{
				ServerID: gs.serverID,
				UserID:   callerID,
			}).
			Return(db.GetServerUserPermissionsRow{HasElevatedPermissions: false}, nil)

		msg := newClientWsMsg(t, gameevents.TypeClearPlayerReports, gameevents.ClearPlayerReportsPayload{PlayerID: uuid.New()})
		gs.handleClientWsMessage(client, msg)
	})
}

func TestHandleEjectPlayer(t *testing.T) {
	t.Run("ejects player when caller has elevated permissions", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		callerID := uuid.New()
		client := newStubClient(t, gs, callerID, Player)
		registerClientOnServer(t, gs, client)

		targetID := uuid.New()

		mocks.Store.EXPECT().
			GetServerUserPermissions(gomock.Any(), db.GetServerUserPermissionsParams{
				ServerID: gs.serverID,
				UserID:   callerID,
			}).
			Return(db.GetServerUserPermissionsRow{HasElevatedPermissions: true}, nil)

		mocks.GfClient.EXPECT().
			EjectPlayer(gomock.Eq(gs.serverID), gomock.Eq(targetID), gomock.Eq(callerID)).
			Times(1)

		msg := newClientWsMsg(t, gameevents.TypeEjectPlayer, gameevents.EjectPlayerPayload{PlayerID: targetID})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("silently ignored when caller lacks permissions", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		callerID := uuid.New()
		client := newStubClient(t, gs, callerID, Player)
		registerClientOnServer(t, gs, client)

		mocks.Store.EXPECT().
			GetServerUserPermissions(gomock.Any(), db.GetServerUserPermissionsParams{
				ServerID: gs.serverID,
				UserID:   callerID,
			}).
			Return(db.GetServerUserPermissionsRow{HasElevatedPermissions: false}, nil)

		msg := newClientWsMsg(t, gameevents.TypeEjectPlayer, gameevents.EjectPlayerPayload{PlayerID: uuid.New()})
		gs.handleClientWsMessage(client, msg)
	})

	t.Run("db error sends error to client", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		callerID := uuid.New()
		client := newStubClient(t, gs, callerID, Player)
		registerClientOnServer(t, gs, client)

		mocks.Store.EXPECT().
			GetServerUserPermissions(gomock.Any(), gomock.Any()).
			Return(db.GetServerUserPermissionsRow{}, fmt.Errorf("db error"))

		msg := newClientWsMsg(t, gameevents.TypeEjectPlayer, gameevents.EjectPlayerPayload{PlayerID: uuid.New()})
		gs.handleClientWsMessage(client, msg)

		requireClientReceivedMsg(t, client)
	})
}
