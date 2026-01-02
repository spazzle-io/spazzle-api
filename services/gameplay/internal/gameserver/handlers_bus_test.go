package gameserver

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	mockgameflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHandleArtistSelected(t *testing.T) {
	artistID := uuid.New()
	correlationID := uuid.New()

	testCases := []struct {
		name          string
		clients       func() []Client
		buildStubs    func(gameServer *GameServer, gfClient *mockgameflowclient.MockClient)
		directMsgSent bool
	}{
		{
			name: "success",
			clients: func() []Client {
				return []Client{
					{
						userID:       artistID,
						connID:       uuid.New(),
						isSpectating: false,
					},
				}
			},
			buildStubs:    func(gameServer *GameServer, gfClient *mockgameflowclient.MockClient) {},
			directMsgSent: true,
		},
		{
			name: "user has no non-spectating clients",
			clients: func() []Client {
				return []Client{
					{
						userID:       artistID,
						connID:       uuid.New(),
						isSpectating: true,
					},
					{
						userID:       artistID,
						connID:       uuid.New(),
						isSpectating: true,
					},
				}
			},
			buildStubs: func(gameServer *GameServer, gfClient *mockgameflowclient.MockClient) {
				gfClient.EXPECT().
					AcknowledgeGameEvent(gomock.Eq(gameServer.serverID), gomock.Eq(gameevents.EventAckPayload{
						CorrelationID: correlationID,
						InstanceID:    gameServer.instanceID,
						Status:        gameevents.AckStatusNotApplicable,
						Reason:        "selected artist not in game server instance",
					})).
					Times(1).
					Return(nil)
			},
			directMsgSent: false,
		},
		{
			name: "send direct ws msg failed",
			clients: func() []Client {
				return []Client{
					{
						userID:       artistID,
						connID:       uuid.New(),
						isSpectating: false,
					},
				}
			},
			buildStubs: func(gameServer *GameServer, gfClient *mockgameflowclient.MockClient) {
				gfClient.EXPECT().
					AcknowledgeGameEvent(gomock.Eq(gameServer.serverID), gomock.Eq(gameevents.EventAckPayload{
						CorrelationID: correlationID,
						InstanceID:    gameServer.instanceID,
						Status:        gameevents.AckStatusFailed,
						Reason:        "failed to send artist selected msg to client",
					})).
					Times(1).
					Return(nil)

				gameServer.isClosed.CompareAndSwap(false, true)
			},
			directMsgSent: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, mockGfClient, gameServer := createTestGameServer(t)
			require.NotEmpty(t, mockGfClient)
			require.NotEmpty(t, gameServer)

			directMsgSentCh := make(chan bool, 1)

			go func() {
				select {
				case <-gameServer.directMsg:
					directMsgSentCh <- true
				case <-time.After(1 * time.Second):
					directMsgSentCh <- false
				}
			}()

			mockGfClient.EXPECT().
				AddPlayers(gomock.Eq(gameServer.serverID), gomock.Any()).
				AnyTimes().
				Return()

			for _, client := range tc.clients() {
				gameServer.addClient(&client)
			}

			tc.buildStubs(gameServer, mockGfClient)

			msg := eventbus.Message{
				Timestamp:      time.Now().UTC(),
				Type:           gameevents.TypeArtistSelected,
				Payload:        []byte{1, 2, 3},
				TargetClientID: artistID,
				CorrelationID:  correlationID,
			}
			gameServer.handleEventBusMessage(context.Background(), msg)

			directMsgSent := <-directMsgSentCh

			require.Equal(t, tc.directMsgSent, directMsgSent)
		})
	}
}

func TestHandleArtistConfirmed(t *testing.T) {
	artistID := uuid.New()

	testCases := []struct {
		name                  string
		msg                   func() eventbus.Message
		buildStubs            func(gameServer *GameServer)
		expectedCurrentArtist uuid.UUID
		msgBroadcasted        bool
	}{
		{
			name: "success",
			msg: func() eventbus.Message {
				unmarshalledPayload := gameevents.ArtistConfirmedPayload{
					ArtistID:    artistID,
					RoundNumber: 1,
				}

				payload, err := json.Marshal(unmarshalledPayload)
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				return eventbus.Message{
					ID:         "meg-id",
					Type:       gameevents.TypeArtistConfirmed,
					Timestamp:  time.Now().UTC(),
					StreamType: eventbus.GameEventsStreamType,
					Payload:    payload,
				}
			},
			buildStubs:            func(gameServer *GameServer) {},
			expectedCurrentArtist: artistID,
			msgBroadcasted:        true,
		},
		{
			name: "invalid payload",
			msg: func() eventbus.Message {
				return eventbus.Message{
					ID:         "meg-id",
					Type:       gameevents.TypeArtistConfirmed,
					Timestamp:  time.Now().UTC(),
					StreamType: eventbus.GameEventsStreamType,
					Payload:    []byte("invalid payload"),
				}
			},
			buildStubs:            func(gameServer *GameServer) {},
			expectedCurrentArtist: uuid.Nil,
			msgBroadcasted:        false,
		},
		{
			name: "could not broadcast message",
			msg: func() eventbus.Message {
				unmarshalledPayload := gameevents.ArtistConfirmedPayload{
					ArtistID:    artistID,
					RoundNumber: 1,
				}

				payload, err := json.Marshal(unmarshalledPayload)
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				return eventbus.Message{
					ID:         "meg-id",
					Type:       gameevents.TypeArtistConfirmed,
					Timestamp:  time.Now().UTC(),
					StreamType: eventbus.GameEventsStreamType,
					Payload:    payload,
				}
			},
			buildStubs: func(gameServer *GameServer) {
				gameServer.isClosed.CompareAndSwap(false, true)
			},
			expectedCurrentArtist: artistID,
			msgBroadcasted:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, gameServer := createTestGameServer(t)
			require.NotEmpty(t, gameServer)

			tc.buildStubs(gameServer)

			msgBroadcastedCh := make(chan bool, 1)

			go func() {
				select {
				case <-gameServer.broadcast:
					msgBroadcastedCh <- true
				case <-time.After(1 * time.Second):
					msgBroadcastedCh <- false
				}
			}()

			gameServer.handleEventBusMessage(context.Background(), tc.msg())

			msgBroadcasted := <-msgBroadcastedCh

			require.Equal(t, tc.msgBroadcasted, msgBroadcasted)
			require.Equal(t, tc.expectedCurrentArtist, gameServer.currentArtist)
		})
	}
}

func TestHandleBeginWordSelection(t *testing.T) {
	selectionID := uuid.New()

	testCases := []struct {
		name                string
		msg                 func() eventbus.Message
		buildStubs          func(gameServer *GameServer)
		expectedSelectionID uuid.UUID
		msgBroadcasted      bool
	}{
		{
			name: "success",
			msg: func() eventbus.Message {
				unmarshalledPayload := gameevents.BeginWordSelectionPayload{
					ArtistID:    uuid.New(),
					RoundNumber: 1,
					EndsAt:      time.Now().UTC().Add(5 * time.Minute),
					SelectionID: selectionID,
				}

				payload, err := json.Marshal(unmarshalledPayload)
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				return eventbus.Message{
					ID:         "meg-id",
					Type:       gameevents.TypeBeginWordSelection,
					Timestamp:  time.Now().UTC(),
					StreamType: eventbus.GameEventsStreamType,
					Payload:    payload,
				}
			},
			buildStubs:          func(gameServer *GameServer) {},
			expectedSelectionID: selectionID,
			msgBroadcasted:      true,
		},
		{
			name: "invalid payload",
			msg: func() eventbus.Message {
				return eventbus.Message{
					ID:         "meg-id",
					Type:       gameevents.TypeBeginWordSelection,
					Timestamp:  time.Now().UTC(),
					StreamType: eventbus.GameEventsStreamType,
					Payload:    []byte("invalid payload"),
				}
			},
			buildStubs:          func(gameServer *GameServer) {},
			expectedSelectionID: uuid.Nil,
			msgBroadcasted:      false,
		},
		{
			name: "could not broadcast message",
			msg: func() eventbus.Message {
				unmarshalledPayload := gameevents.BeginWordSelectionPayload{
					ArtistID:    uuid.New(),
					RoundNumber: 1,
					EndsAt:      time.Now().UTC().Add(5 * time.Minute),
					SelectionID: selectionID,
				}

				payload, err := json.Marshal(unmarshalledPayload)
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				return eventbus.Message{
					ID:         "meg-id",
					Type:       gameevents.TypeBeginWordSelection,
					Timestamp:  time.Now().UTC(),
					StreamType: eventbus.GameEventsStreamType,
					Payload:    payload,
				}
			},
			buildStubs: func(gameServer *GameServer) {
				gameServer.isClosed.CompareAndSwap(false, true)
			},
			expectedSelectionID: selectionID,
			msgBroadcasted:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, gameServer := createTestGameServer(t)
			require.NotEmpty(t, gameServer)

			tc.buildStubs(gameServer)

			msgBroadcastedCh := make(chan bool, 1)

			go func() {
				select {
				case <-gameServer.broadcast:
					msgBroadcastedCh <- true
				case <-time.After(1 * time.Second):
					msgBroadcastedCh <- false
				}
			}()

			gameServer.handleEventBusMessage(context.Background(), tc.msg())

			msgBroadcasted := <-msgBroadcastedCh

			require.Equal(t, tc.msgBroadcasted, msgBroadcasted)
			require.Equal(t, tc.expectedSelectionID, gameServer.workflowSelectionID)
		})
	}
}

func TestHandleWordSelected(t *testing.T) {
	selectedWord := "selected word"

	testCases := []struct {
		name                string
		msg                 func() eventbus.Message
		buildStubs          func(gameServer *GameServer, mockGfClient *mockgameflowclient.MockClient)
		expectedCurrentWord string
		msgBroadcasted      bool
	}{
		{
			name: "success",
			msg: func() eventbus.Message {
				unmarshalledPayload := gameevents.WordSelectedPayload{
					RoundNumber: 1,
				}

				payload, err := json.Marshal(unmarshalledPayload)
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				return eventbus.Message{
					ID:         "meg-id",
					Type:       gameevents.TypeWordSelected,
					Timestamp:  time.Now().UTC(),
					StreamType: eventbus.GameEventsStreamType,
					Payload:    payload,
				}
			},
			buildStubs: func(gameServer *GameServer, mockGfClient *mockgameflowclient.MockClient) {
				mockGfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{
						CurrentWord: types.Word{
							Text: selectedWord,
						},
					}, nil)
			},
			expectedCurrentWord: selectedWord,
			msgBroadcasted:      true,
		},
		{
			name: "could not fetch game state",
			msg: func() eventbus.Message {
				unmarshalledPayload := gameevents.WordSelectedPayload{
					RoundNumber: 1,
				}

				payload, err := json.Marshal(unmarshalledPayload)
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				return eventbus.Message{
					ID:         "meg-id",
					Type:       gameevents.TypeWordSelected,
					Timestamp:  time.Now().UTC(),
					StreamType: eventbus.GameEventsStreamType,
					Payload:    payload,
				}
			},
			buildStubs: func(gameServer *GameServer, mockGfClient *mockgameflowclient.MockClient) {
				mockGfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{}, errors.New("failed to fetch game state"))
			},
			expectedCurrentWord: "",
			msgBroadcasted:      false,
		},
		{
			name: "could not broadcast message",
			msg: func() eventbus.Message {
				unmarshalledPayload := gameevents.WordSelectedPayload{
					RoundNumber: 1,
				}

				payload, err := json.Marshal(unmarshalledPayload)
				require.NoError(t, err)
				require.NotEmpty(t, payload)

				return eventbus.Message{
					ID:         "meg-id",
					Type:       gameevents.TypeWordSelected,
					Timestamp:  time.Now().UTC(),
					StreamType: eventbus.GameEventsStreamType,
					Payload:    payload,
				}
			},
			buildStubs: func(gameServer *GameServer, mockGfClient *mockgameflowclient.MockClient) {
				mockGfClient.EXPECT().
					GetGameState(gomock.Any()).
					Times(1).
					Return(&types.GameStateView{
						CurrentWord: types.Word{
							Text: selectedWord,
						},
					}, nil)

				gameServer.isClosed.CompareAndSwap(false, true)
			},
			expectedCurrentWord: selectedWord,
			msgBroadcasted:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, mockGfClient, gameServer := createTestGameServer(t)
			require.NotEmpty(t, mockGfClient)
			require.NotEmpty(t, gameServer)

			tc.buildStubs(gameServer, mockGfClient)

			msgBroadcastedCh := make(chan bool, 1)

			go func() {
				select {
				case <-gameServer.broadcast:
					msgBroadcastedCh <- true
				case <-time.After(1 * time.Second):
					msgBroadcastedCh <- false
				}
			}()

			gameServer.handleEventBusMessage(context.Background(), tc.msg())

			msgBroadcasted := <-msgBroadcastedCh

			require.Equal(t, tc.msgBroadcasted, msgBroadcasted)
			require.Equal(t, tc.expectedCurrentWord, gameServer.currentWord)
		})
	}
}

func TestUserHasNonSpectatingClients(t *testing.T) {
	userID := uuid.New()

	testCases := []struct {
		name                    string
		clients                 func() []Client
		hasNonSpectatingClients bool
	}{
		{
			name: "has non-spectating clients",
			clients: func() []Client {
				return []Client{
					{
						userID:       userID,
						connID:       uuid.New(),
						isSpectating: true,
					},
					{
						userID:       userID,
						connID:       uuid.New(),
						isSpectating: false,
					},
				}
			},
			hasNonSpectatingClients: true,
		},
		{
			name: "has no non-spectating clients",
			clients: func() []Client {
				return []Client{
					{
						userID:       userID,
						connID:       uuid.New(),
						isSpectating: true,
					},
					{
						userID:       userID,
						connID:       uuid.New(),
						isSpectating: true,
					},
				}
			},
			hasNonSpectatingClients: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, mockGfClient, gameServer := createTestGameServer(t)
			require.NotEmpty(t, mockGfClient)
			require.NotEmpty(t, gameServer)

			mockGfClient.EXPECT().
				AddPlayers(gomock.Eq(gameServer.serverID), gomock.Any()).
				AnyTimes().
				Return()

			for _, client := range tc.clients() {
				gameServer.addClient(&client)
			}

			hasNonSpectatingClients := gameServer.userHasNonSpectatingClients(userID)
			require.Equal(t, tc.hasNonSpectatingClients, hasNonSpectatingClients)
		})
	}
}

func TestSetCurrentArtist(t *testing.T) {
	artistID := uuid.New()

	_, _, _, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	require.Empty(t, gameServer.getCurrentArtist())

	gameServer.setCurrentArtist(artistID)
	require.Equal(t, artistID, gameServer.getCurrentArtist())
}

func TestSetCurrentWord(t *testing.T) {
	currentWord := "word selected"

	_, _, _, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	require.Empty(t, gameServer.getCurrentWord())

	gameServer.setCurrentWord(currentWord)
	require.Equal(t, currentWord, gameServer.getCurrentWord())
}

func TestSetWorkflowSelectionID(t *testing.T) {
	selectionID := uuid.New()

	_, _, _, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)

	require.Empty(t, gameServer.getWorkflowSelectionID())

	gameServer.setWorkflowSelectionID(selectionID)
	require.Equal(t, selectionID, gameServer.getWorkflowSelectionID())
}
