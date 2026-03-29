package gameserver

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.uber.org/mock/gomock"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/stretchr/testify/require"
)

func newBusMsg(t *testing.T, msgType string, payload any) eventbus.Message {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	return eventbus.Message{
		ID:         uuid.NewString(),
		Type:       msgType,
		Timestamp:  time.Now().UTC(),
		StreamType: eventbus.GameEventsStreamType,
		Payload:    raw,
	}
}

func newBusMsgWithTarget(t *testing.T, msgType string, payload any, target uuid.UUID) eventbus.Message {
	t.Helper()
	msg := newBusMsg(t, msgType, payload)
	msg.TargetClientID = target
	msg.CorrelationID = uuid.New()
	return msg
}

func invalidPayloadMsg(msgType string) eventbus.Message {
	return eventbus.Message{
		ID:         uuid.NewString(),
		Type:       msgType,
		Timestamp:  time.Now().UTC(),
		StreamType: eventbus.GameEventsStreamType,
		Payload:    []byte(`{invalid`),
	}
}

func awaitBroadcast(gs *GameServer, timeout time.Duration) bool {
	select {
	case <-gs.broadcast:
		return true
	case <-time.After(timeout):
		return false
	}
}

func awaitDirectMsg(gs *GameServer, targetUserID uuid.UUID, timeout time.Duration) bool {
	select {
	case msg := <-gs.directMsg:
		for _, recipient := range msg.Recipients {
			if recipient.UserID == targetUserID {
				return true
			}
		}
		return false
	case <-time.After(timeout):
		return false
	}
}

func TestHandlePlayersJoined(t *testing.T) {
	t.Run("adds players and removes rejected ones", func(t *testing.T) {
		_, gs := createTestGameServer(t)

		kept := uuid.New()
		rejected := uuid.New()

		msg := newBusMsg(t, gameevents.TypePlayersJoined, gameevents.PlayersJoinedPayload{
			AddedPlayers: []uuid.UUID{kept, rejected},
			RejectedPlayers: []gameevents.RejectedPlayer{
				{PlayerID: rejected, Reason: gameevents.RejectionReasonEjectedPlayer},
			},
		})

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Len(t, gs.activePlayers, 1)
		require.True(t, gs.activePlayers[kept])
		require.False(t, gs.activePlayers[rejected])
	})

	t.Run("all players rejected leaves none active", func(t *testing.T) {
		_, gs := createTestGameServer(t)

		player := uuid.New()
		msg := newBusMsg(t, gameevents.TypePlayersJoined, gameevents.PlayersJoinedPayload{
			AddedPlayers: []uuid.UUID{player},
			RejectedPlayers: []gameevents.RejectedPlayer{
				{PlayerID: player, Reason: gameevents.RejectionReasonEjectedPlayer},
			},
		})

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Empty(t, gs.activePlayers)
	})

	t.Run("invalid payload does not broadcast or mutate state", func(t *testing.T) {
		_, gs := createTestGameServer(t)

		gs.handleEventBusMessage(context.Background(), invalidPayloadMsg(gameevents.TypePlayersJoined))

		require.False(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Empty(t, gs.activePlayers)
	})
}

func TestHandlePlayersLeft(t *testing.T) {
	t.Run("removes players that were active", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		gs.mu.RLock()
		var activePlayers []uuid.UUID
		for id := range gs.activePlayers {
			activePlayers = append(activePlayers, id)
		}
		gs.mu.RUnlock()
		require.Len(t, activePlayers, 2)

		newPlayer := uuid.New()
		gs.mu.Lock()
		gs.activePlayers[newPlayer] = true
		gs.mu.Unlock()

		msg := newBusMsg(t, gameevents.TypePlayersLeft, gameevents.PlayersLeftPayload{
			PlayerIDs: activePlayers,
		})

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Len(t, gs.activePlayers, 1)
		require.True(t, gs.activePlayers[newPlayer])
	})

	t.Run("leaving unknown player is a harmless no-op", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		msg := newBusMsg(t, gameevents.TypePlayersLeft, gameevents.PlayersLeftPayload{
			PlayerIDs: []uuid.UUID{uuid.New()}, // not in active players
		})

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Len(t, gs.activePlayers, 2)
	})

	t.Run("invalid payload does not broadcast or mutate state", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		gs.handleEventBusMessage(context.Background(), invalidPayloadMsg(gameevents.TypePlayersLeft))

		require.False(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Len(t, gs.activePlayers, 2)
	})
}

func TestHandlePlayersEjected(t *testing.T) {
	t.Run("removes ejected player and keeps others", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		ejected := uuid.New()
		gs.mu.Lock()
		gs.activePlayers[ejected] = true
		gs.mu.Unlock()

		msg := newBusMsg(t, gameevents.TypePlayersEjected, gameevents.PlayersEjectedPayload{
			CurrentRound: 1,
			Ejections: []gameevents.PlayerEjection{
				{PlayerID: ejected, IsArtist: false, EjectorID: uuid.New(), TotalReports: 3},
			},
		})

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Len(t, gs.activePlayers, 2)
		require.False(t, gs.activePlayers[ejected])
	})

	t.Run("invalid payload does not broadcast or mutate state", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		gs.handleEventBusMessage(context.Background(), invalidPayloadMsg(gameevents.TypePlayersEjected))

		require.False(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Len(t, gs.activePlayers, 2)
	})
}

func TestHandleArtistSelected(t *testing.T) {
	t.Run("sends direct message when target has non-spectating client", func(t *testing.T) {
		_, gs := createTestGameServer(t)

		artistID := uuid.New()
		gs.mu.Lock()
		gs.clients[artistID] = map[uuid.UUID]*Client{
			uuid.New(): newStubClient(t, gs, uuid.New(), Player),
		}
		gs.mu.Unlock()

		msg := newBusMsgWithTarget(t, gameevents.TypeArtistSelected, struct{}{}, artistID)

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitDirectMsg(gs, artistID, broadcastTimeout))
	})

	t.Run("acks NotApplicable when target client is absent", func(t *testing.T) {
		mocks, gs := createTestGameServer(t)

		artistID := uuid.New()

		msg := newBusMsgWithTarget(t, gameevents.TypeArtistSelected, struct{}{}, artistID)

		mocks.GfClient.EXPECT().
			AcknowledgeGameEvent(gomock.Eq(gs.serverID), gomock.Eq(gameevents.EventAckPayload{
				CorrelationID: msg.CorrelationID,
				InstanceID:    gs.instanceID,
				Status:        gameevents.AckStatusNotApplicable,
				Reason:        "selected artist not in game server instance",
			})).
			Times(1).
			Return(nil)

		go gs.handleEventBusMessage(context.Background(), msg)

		require.False(t, awaitDirectMsg(gs, artistID, broadcastTimeout))
	})

	t.Run("acks NotApplicable when all target clients are spectating", func(t *testing.T) {
		mocks, gs := createTestGameServer(t)

		artistID := uuid.New()

		gs.mu.Lock()
		gs.clients[artistID] = map[uuid.UUID]*Client{
			uuid.New(): newStubClient(t, gs, uuid.New(), Spectator),
			uuid.New(): newStubClient(t, gs, uuid.New(), Spectator),
		}
		gs.mu.Unlock()

		msg := newBusMsgWithTarget(t, gameevents.TypeArtistSelected, struct{}{}, artistID)

		mocks.GfClient.EXPECT().
			AcknowledgeGameEvent(gomock.Eq(gs.serverID), gomock.Eq(gameevents.EventAckPayload{
				CorrelationID: msg.CorrelationID,
				InstanceID:    gs.instanceID,
				Status:        gameevents.AckStatusNotApplicable,
				Reason:        "selected artist not in game server instance",
			})).
			Times(1).
			Return(nil)

		go gs.handleEventBusMessage(context.Background(), msg)

		require.False(t, awaitDirectMsg(gs, artistID, broadcastTimeout))
	})
}

func TestHandleArtistConfirmed(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, gs := createTestGameServer(t)

		artistID := uuid.New()
		msg := newBusMsg(t, gameevents.TypeArtistConfirmed, gameevents.ArtistConfirmedPayload{
			ArtistID:     artistID,
			CurrentRound: 3,
		})

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Equal(t, artistID, gs.currentArtist)
		require.Equal(t, uint8(3), gs.currentRound)
	})

	t.Run("invalid payload does not broadcast or mutate state", func(t *testing.T) {
		_, gs := createTestGameServer(t)

		gs.handleEventBusMessage(context.Background(), invalidPayloadMsg(gameevents.TypeArtistConfirmed))

		require.False(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Equal(t, uuid.Nil, gs.currentArtist)
		require.Equal(t, uint8(0), gs.currentRound)
	})
}

func TestHandleNextArtistSelected(t *testing.T) {
	t.Run("sends direct message when target is present and non-spectating", func(t *testing.T) {
		_, gs := createTestGameServer(t)

		nextArtist := uuid.New()
		gs.mu.Lock()
		gs.clients[nextArtist] = map[uuid.UUID]*Client{
			uuid.New(): newStubClient(t, gs, uuid.New(), Player),
		}
		gs.mu.Unlock()

		msg := newBusMsgWithTarget(t, gameevents.TypeNextArtistSelected, struct{}{}, nextArtist)

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitDirectMsg(gs, nextArtist, broadcastTimeout))
	})

	t.Run("acks NotApplicable when target is absent", func(t *testing.T) {
		mocks, gs := createTestGameServer(t)

		nextArtist := uuid.New()

		msg := newBusMsgWithTarget(t, gameevents.TypeNextArtistSelected, struct{}{}, nextArtist)

		mocks.GfClient.EXPECT().
			AcknowledgeGameEvent(gomock.Eq(gs.serverID), gomock.Eq(gameevents.EventAckPayload{
				CorrelationID: msg.CorrelationID,
				InstanceID:    gs.instanceID,
				Status:        gameevents.AckStatusNotApplicable,
				Reason:        "next artist selected not in game server instance",
			})).
			Times(1).
			Return(nil)

		go gs.handleEventBusMessage(context.Background(), msg)

		require.False(t, awaitDirectMsg(gs, nextArtist, broadcastTimeout))
	})

	t.Run("acks NotApplicable when all target clients are spectating", func(t *testing.T) {
		mocks, gs := createTestGameServer(t)

		nextArtist := uuid.New()

		gs.mu.Lock()
		gs.clients[nextArtist] = map[uuid.UUID]*Client{
			uuid.New(): newStubClient(t, gs, uuid.New(), Spectator),
			uuid.New(): newStubClient(t, gs, uuid.New(), Spectator),
		}
		gs.mu.Unlock()

		msg := newBusMsgWithTarget(t, gameevents.TypeNextArtistSelected, struct{}{}, nextArtist)

		mocks.GfClient.EXPECT().
			AcknowledgeGameEvent(gomock.Eq(gs.serverID), gomock.Eq(gameevents.EventAckPayload{
				CorrelationID: msg.CorrelationID,
				InstanceID:    gs.instanceID,
				Status:        gameevents.AckStatusNotApplicable,
				Reason:        "next artist selected not in game server instance",
			})).
			Times(1).
			Return(nil)

		go gs.handleEventBusMessage(context.Background(), msg)

		require.False(t, awaitDirectMsg(gs, nextArtist, broadcastTimeout))
	})
}

func TestHandleArtistDisconnected(t *testing.T) {
	t.Run("clears current artist and removes from active players", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		gs.mu.RLock()
		artistID := gs.currentArtist
		gs.mu.RUnlock()
		require.NotNil(t, artistID)

		gs.mu.Lock()
		gs.activePlayers[artistID] = true
		gs.mu.Unlock()

		msg := newBusMsg(t, gameevents.TypeArtistDisconnected, gameevents.ArtistDisconnectedPayload{
			ArtistID: artistID,
		})

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Equal(t, uuid.Nil, gs.currentArtist)
		require.False(t, gs.activePlayers[artistID])
		require.Len(t, gs.activePlayers, 2)
	})

	t.Run("invalid payload does not broadcast or mutate state", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		gs.mu.RLock()
		originalArtist := gs.currentArtist
		gs.mu.RUnlock()

		gs.handleEventBusMessage(context.Background(), invalidPayloadMsg(gameevents.TypeArtistDisconnected))

		require.False(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Equal(t, originalArtist, gs.currentArtist)
	})
}

func TestHandleWordSelected(t *testing.T) {
	t.Run("fetches game state and sets current word", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		expectedWord := "mjAne"

		mocks.GfClient.EXPECT().
			GetGameState(gs.serverID).
			Return(&types.GameStateView{
				CurrentWord: types.Word{Text: expectedWord},
			}, nil)

		msg := newBusMsg(t, gameevents.TypeWordSelected, struct{}{})

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Equal(t, expectedWord, gs.currentWord)
	})

	t.Run("does not broadcast or update word when GetGameState fails", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		mocks.GfClient.EXPECT().
			GetGameState(gs.serverID).
			Return(nil, fmt.Errorf("could not get game state"))

		msg := newBusMsg(t, gameevents.TypeWordSelected, struct{}{})

		go gs.handleEventBusMessage(context.Background(), msg)

		require.False(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Equal(t, "current word", gs.currentWord)
	})
}

func TestHandleRoundEnded(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, gs := createInitializedTestGameServer(t)

		gs.mu.Lock()
		gs.correctGuessers[uuid.New()] = true
		gs.correctGuessers[uuid.New()] = true
		gs.mu.Unlock()

		gs.mu.RLock()
		require.NotEqual(t, uuid.Nil, gs.currentArtist)
		require.NotEmpty(t, gs.currentWord)
		require.Len(t, gs.correctGuessers, 2)
		playerCount := len(gs.activePlayers)
		gs.mu.RUnlock()

		msg := newBusMsg(t, gameevents.TypeRoundEnded, struct{}{})

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitBroadcast(gs, broadcastTimeout))

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Empty(t, gs.currentArtist)
		require.Empty(t, gs.currentWord)
		require.Empty(t, gs.correctGuessers)
		require.Len(t, gs.activePlayers, playerCount)
	})
}

func TestHandleGameEnded(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mocks, gs := createInitializedTestGameServer(t)

		mocks.Session.EXPECT().Close().Times(1)

		require.True(t, gs.isGameActive.Load())

		gs.mu.RLock()
		require.NotEmpty(t, gs.activePlayers)
		require.NotNil(t, gs.busSession)
		gs.mu.RUnlock()

		msg := newBusMsg(t, gameevents.TypeGameEnded, struct{}{})

		go gs.handleEventBusMessage(context.Background(), msg)

		require.True(t, awaitBroadcast(gs, broadcastTimeout))

		time.Sleep(100 * time.Millisecond)

		require.False(t, gs.isGameActive.Load())

		gs.mu.RLock()
		defer gs.mu.RUnlock()
		require.Empty(t, gs.activePlayers)
		require.Nil(t, gs.busSession)
	})
}

func TestBroadcastPassthroughEvents(t *testing.T) {
	passthroughTypes := []string{
		gameevents.TypePlayerWarned,
		gameevents.TypePlayersReported,
		gameevents.TypePlayerReportsCleared,
		gameevents.TypeBeginWordSelection,
		gameevents.TypeWordHintRevealed,
		gameevents.TypeWordGuessed,
		gameevents.TypeBeginDrawing,
		gameevents.TypeEndDrawing,
	}

	for _, msgType := range passthroughTypes {
		t.Run(msgType, func(t *testing.T) {
			_, gs := createTestGameServer(t)

			msg := eventbus.Message{
				ID:         uuid.NewString(),
				Type:       msgType,
				Timestamp:  time.Now().UTC(),
				StreamType: eventbus.GameEventsStreamType,
				Payload:    []byte(`{}`),
			}

			go gs.handleEventBusMessage(context.Background(), msg)

			require.True(t, awaitBroadcast(gs, broadcastTimeout))
		})
	}
}
