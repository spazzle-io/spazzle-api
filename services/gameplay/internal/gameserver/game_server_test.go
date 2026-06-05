package gameserver

import (
	"errors"
	"math/big"
	"testing"
	"time"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	mockcache "github.com/spazzle-io/spazzle-api/libs/common/cache/mock"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	mockeventbus "github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	mockgameflowclient "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	mockwordstore "github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type MockConfig struct {
	Store     *mockdb.MockStore
	Cache     *mockcache.MockCache
	GameCache *gamecache.GameCache
	Bus       *mockeventbus.MockEventBus
	Session   *mockeventbus.MockSession
	GfClient  *mockgameflowclient.MockClient
	WordStore *mockwordstore.MockStore
}

func createTestGameServer(t *testing.T) (*MockConfig, *GameServer) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	serverID := uuid.New()

	env := &util.Config{
		AppConfig: commonConfig.AppConfig{
			ServiceName: "test",
			Environment: commonConfig.Development,
		},
	}

	mockCache := mockcache.NewMockCache(ctrl)

	mockConfig := &MockConfig{
		Store:     mockdb.NewMockStore(ctrl),
		Cache:     mockCache,
		GameCache: gamecache.New(env, mockCache),
		Bus:       mockeventbus.NewMockEventBus(ctrl),
		Session:   mockeventbus.NewMockSession(ctrl),
		GfClient:  mockgameflowclient.NewMockClient(ctrl),
		WordStore: mockwordstore.NewMockStore(ctrl),
	}

	config := &Config{
		Env:       env,
		Store:     mockConfig.Store,
		Cache:     mockConfig.Cache,
		GameCache: mockConfig.GameCache,
		Bus:       mockConfig.Bus,
		GfClient:  mockConfig.GfClient,
		WordStore: mockConfig.WordStore,
	}

	gameServer, err := NewGameServer(serverID, config, WithoutBackgroundWorkers())
	require.NoError(t, err)
	require.NotEmpty(t, gameServer)
	require.Equal(t, serverID, gameServer.serverID)
	require.NotEmpty(t, gameServer.instanceID)
	require.Empty(t, gameServer.gameID)

	return mockConfig, gameServer
}

func createInitializedTestGameServer(t *testing.T) (*MockConfig, *GameServer) {
	t.Helper()

	mockConfig, gameServer := createTestGameServer(t)
	require.NotNil(t, gameServer)

	gameServer.gameID = uuid.New()
	gameServer.currentRound = 1
	gameServer.currentArtist = uuid.New()
	gameServer.currentWord = "current word"
	gameServer.activePlayers = map[uuid.UUID]bool{
		uuid.New(): true,
		uuid.New(): true,
	}
	gameServer.busSession = mockConfig.Session
	gameServer.isGameActive.Store(true)

	return mockConfig, gameServer
}

func TestCreateGameServer(t *testing.T) {
	_, gameServer := createTestGameServer(t)
	require.NotEmpty(t, gameServer)
}

func TestInitializeGame(t *testing.T) {
	gameID := uuid.New()

	dbServer := db.Server{
		NumRoundsPerGame:  10,
		RoundDurationSecs: 75,
		StakePerGame: pgtype.Numeric{
			Int:   big.NewInt(1000000),
			Exp:   5,
			Valid: true,
		},
	}

	gameInput := types.GameInput{
		NumRounds:       10,
		DrawingDuration: 75 * time.Second,
		StakePerGame:    "100000000000",
	}

	gameState := &types.GameStateView{
		CurrentRound:  1,
		CurrentArtist: uuid.New(),
		CurrentWord: types.Word{
			Text: "current word",
		},
		NumActivePlayers: 2,
	}

	testCases := []struct {
		name       string
		buildStubs func(gameServer *GameServer, config *MockConfig)
		shouldErr  bool
	}{
		{
			name: "success",
			buildStubs: func(gameServer *GameServer, config *MockConfig) {
				config.Store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(gameServer.serverID)).
					Times(1).
					Return(dbServer, nil)

				config.GfClient.EXPECT().
					Game(gomock.Eq(gameServer.serverID), EqGameInput(gameInput)).
					Times(1).
					Return(gameID, nil)

				config.GfClient.EXPECT().
					HeartbeatGameServerInstance(gomock.Eq(gameServer.serverID), gomock.Eq(gameID), gomock.Eq(gameServer.instanceID)).
					Times(1).
					Return(nil)

				config.GfClient.EXPECT().
					GetGameState(gomock.Eq(gameServer.serverID)).
					Times(1).
					Return(gameState, nil)

				config.Bus.EXPECT().
					Session(gomock.Eq(eventbus.GameIdentifier{
						GameID:       gameID,
						GameServerID: gameServer.serverID,
					})).
					Times(1).
					Return(config.Session, nil)

				config.Session.EXPECT().
					Subscribe(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				config.Session.EXPECT().
					Subscribe(gomock.Any(), gomock.Eq(eventbus.DrawingUpdatesStreamType), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			shouldErr: false,
		},
		{
			name: "failed to get db server",
			buildStubs: func(gameServer *GameServer, config *MockConfig) {
				config.Store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(gameServer.serverID)).
					Times(1).
					Return(db.Server{}, errors.New("failed to get db server"))
			},
			shouldErr: true,
		},
		{
			name: "failed to parse stake per game",
			buildStubs: func(gameServer *GameServer, config *MockConfig) {
				dbServer := db.Server{
					StakePerGame: pgtype.Numeric{
						Int:   big.NewInt(1000000),
						Exp:   5,
						Valid: false,
					},
				}

				config.Store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(gameServer.serverID)).
					Times(1).
					Return(dbServer, nil)
			},
			shouldErr: true,
		},
		{
			name: "could not get or create game",
			buildStubs: func(gameServer *GameServer, config *MockConfig) {
				config.Store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(gameServer.serverID)).
					Times(1).
					Return(dbServer, nil)

				config.GfClient.EXPECT().
					Game(gomock.Eq(gameServer.serverID), EqGameInput(gameInput)).
					Times(1).
					Return(uuid.Nil, errors.New("could not get or create game"))
			},
			shouldErr: true,
		},
		{
			name: "failed to register game server instance",
			buildStubs: func(gameServer *GameServer, config *MockConfig) {
				config.Store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(gameServer.serverID)).
					Times(1).
					Return(dbServer, nil)

				config.GfClient.EXPECT().
					Game(gomock.Eq(gameServer.serverID), EqGameInput(gameInput)).
					Times(1).
					Return(gameID, nil)

				config.GfClient.EXPECT().
					HeartbeatGameServerInstance(gomock.Eq(gameServer.serverID), gomock.Eq(gameID), gomock.Eq(gameServer.instanceID)).
					Times(1).
					Return(errors.New("failed to register game server instance"))
			},
			shouldErr: true,
		},
		{
			name: "failed to get game state",
			buildStubs: func(gameServer *GameServer, config *MockConfig) {
				config.Store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(gameServer.serverID)).
					Times(1).
					Return(dbServer, nil)

				config.GfClient.EXPECT().
					Game(gomock.Eq(gameServer.serverID), EqGameInput(gameInput)).
					Times(1).
					Return(gameID, nil)

				config.GfClient.EXPECT().
					HeartbeatGameServerInstance(gomock.Eq(gameServer.serverID), gomock.Eq(gameID), gomock.Eq(gameServer.instanceID)).
					Times(1).
					Return(nil)

				config.Bus.EXPECT().
					Session(gomock.Eq(eventbus.GameIdentifier{
						GameID:       gameID,
						GameServerID: gameServer.serverID,
					})).
					Times(1).
					Return(config.Session, nil)

				config.Session.EXPECT().
					Subscribe(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).
					Return(nil)

				config.GfClient.EXPECT().
					GetGameState(gomock.Eq(gameServer.serverID)).
					Times(1).
					Return(nil, errors.New("failed to get game state"))

				config.Session.EXPECT().
					Close().
					Times(1).
					Return()
			},
			shouldErr: true,
		},
		{
			name: "failed to get bus session",
			buildStubs: func(gameServer *GameServer, config *MockConfig) {
				config.Store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(gameServer.serverID)).
					Times(1).
					Return(dbServer, nil)

				config.GfClient.EXPECT().
					Game(gomock.Eq(gameServer.serverID), EqGameInput(gameInput)).
					Times(1).
					Return(gameID, nil)

				config.GfClient.EXPECT().
					HeartbeatGameServerInstance(gomock.Eq(gameServer.serverID), gomock.Eq(gameID), gomock.Eq(gameServer.instanceID)).
					Times(1).
					Return(nil)

				config.Bus.EXPECT().
					Session(gomock.Eq(eventbus.GameIdentifier{
						GameID:       gameID,
						GameServerID: gameServer.serverID,
					})).
					Times(1).
					Return(nil, errors.New("failed to get bus session"))
			},
			shouldErr: true,
		},
		{
			name: "failed to subscribe to game events stream",
			buildStubs: func(gameServer *GameServer, config *MockConfig) {
				config.Store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(gameServer.serverID)).
					Times(1).
					Return(dbServer, nil)

				config.GfClient.EXPECT().
					Game(gomock.Eq(gameServer.serverID), EqGameInput(gameInput)).
					Times(1).
					Return(gameID, nil)

				config.GfClient.EXPECT().
					HeartbeatGameServerInstance(gomock.Eq(gameServer.serverID), gomock.Eq(gameID), gomock.Eq(gameServer.instanceID)).
					Times(1).
					Return(nil)

				config.Bus.EXPECT().
					Session(gomock.Eq(eventbus.GameIdentifier{
						GameID:       gameID,
						GameServerID: gameServer.serverID,
					})).
					Times(1).
					Return(config.Session, nil)

				config.Session.EXPECT().
					Subscribe(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("failed to subscribe to game events stream"))

				config.Session.EXPECT().
					Close().
					Times(1).
					Return()
			},
			shouldErr: true,
		},
		{
			name: "failed to subscribe to drawing updates stream",
			buildStubs: func(gameServer *GameServer, config *MockConfig) {
				config.Store.EXPECT().
					GetServerById(gomock.Any(), gomock.Eq(gameServer.serverID)).
					Times(1).
					Return(dbServer, nil)

				config.GfClient.EXPECT().
					Game(gomock.Eq(gameServer.serverID), EqGameInput(gameInput)).
					Times(1).
					Return(gameID, nil)

				config.GfClient.EXPECT().
					HeartbeatGameServerInstance(gomock.Eq(gameServer.serverID), gomock.Eq(gameID), gomock.Eq(gameServer.instanceID)).
					Times(1).
					Return(nil)

				config.Bus.EXPECT().
					Session(gomock.Eq(eventbus.GameIdentifier{
						GameID:       gameID,
						GameServerID: gameServer.serverID,
					})).
					Times(1).
					Return(config.Session, nil)

				config.Session.EXPECT().
					Subscribe(gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				config.Session.EXPECT().
					Subscribe(gomock.Any(), gomock.Eq(eventbus.DrawingUpdatesStreamType), gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("failed to subscribe to drawing updates stream"))

				config.Session.EXPECT().
					Close().
					Times(1).
					Return()
			},
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockConfig, gameServer := createTestGameServer(t)
			require.NotNil(t, gameServer)

			tc.buildStubs(gameServer, mockConfig)

			err := gameServer.InitializeGame()
			if tc.shouldErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, gameServer.gameID)
			require.NotEmpty(t, gameServer.currentRound)
			require.NotEmpty(t, gameServer.currentArtist)
			require.NotEmpty(t, gameServer.currentWord)
			require.Empty(t, gameServer.activePlayers)
			require.Empty(t, gameServer.correctGuessers)
			require.NotEmpty(t, gameServer.busSession)
			require.True(t, gameServer.isGameActive.Load())
		})
	}
}

func TestInitializeGame_GameIsActive(t *testing.T) {
	_, gameServer := createInitializedTestGameServer(t)
	err := gameServer.InitializeGame()
	require.NoError(t, err)
}

func TestAddClient(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)
	client := newStubClient(t, gameServer, uuid.New(), Player)

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client.userID], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))

	connectionInfoMsg := <-client.send
	require.Equal(t, gameevents.TypeConnectionInfo, connectionInfoMsg.Type)
	require.NotEmpty(t, connectionInfoMsg.Payload)
	require.False(t, connectionInfoMsg.RequiresWorkflowAck)
	require.Empty(t, connectionInfoMsg.CorrelationID)
}

func TestAddClient_IsSpectating(t *testing.T) {
	_, gameServer := createInitializedTestGameServer(t)

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan *OutgoingMessage, ClientSendChanBufSize),
	}
	client.role.Store(Spectator)

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client.userID], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))

	gameServer.addClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))
}

func TestRemoveClient(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)
	client := newStubClient(t, gameServer, uuid.New(), Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))

	mockConfig.GfClient.EXPECT().
		RemovePlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client.userID], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestRemoveClient_IsArtist(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)
	client := newStubClient(t, gameServer, uuid.New(), Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.currentArtist = client.userID
	gameServer.addClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))

	mockConfig.GfClient.EXPECT().
		ArtistDisconnected(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq(uint8(1)), gomock.Eq(client.userID)).
		Times(1).
		Return(nil)

	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client.userID], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestRemoveClient_IsSpectating(t *testing.T) {
	_, gameServer := createInitializedTestGameServer(t)

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan *OutgoingMessage, ClientSendChanBufSize),
	}
	client.role.Store(Spectator)

	gameServer.addClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))

	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client.userID], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestRemoveClient_UserIdNotRegistered(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)
	client := newStubClient(t, gameServer, uuid.New(), Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	originalClientUserID := client.userID
	client.userID = uuid.New()
	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[originalClientUserID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))
}

func TestRemoveClient_UserConnNotRegistered(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)
	client := newStubClient(t, gameServer, uuid.New(), Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	client.connID = uuid.New()
	gameServer.removeClient(client)

	require.Len(t, gameServer.clients, 1)
	require.Len(t, gameServer.clients[client.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(1))
	require.Equal(t, gameServer.connCount.Load(), int32(1))
}

func TestRemoveAllClients(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)
	client1 := newStubClient(t, gameServer, uuid.New(), Player)

	client2 := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan *OutgoingMessage, ClientSendChanBufSize),
	}
	client2.role.Store(Spectator)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client1.userID})).
		Times(1).
		Return()

	gameServer.addClient(client1)
	gameServer.addClient(client2)

	require.Len(t, gameServer.clients, 2)
	require.Len(t, gameServer.clients[client1.userID], 1)
	require.Len(t, gameServer.clients[client2.userID], 1)
	require.Equal(t, gameServer.clientCount.Load(), int32(2))
	require.Equal(t, gameServer.connCount.Load(), int32(2))

	mockConfig.GfClient.EXPECT().
		RemovePlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client1.userID})).
		Times(1).
		Return()

	gameServer.removeAllClients()

	require.Len(t, gameServer.clients, 0)
	require.Len(t, gameServer.clients[client1.userID], 0)
	require.Len(t, gameServer.clients[client2.userID], 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestDispatchMsg(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)
	client := newStubClient(t, gameServer, uuid.New(), Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	msgType := "msg-type"
	msg := []byte("test msg")

	gameServer.dispatchMsg(&OutgoingMessage{
		WsMessage: WsMessage{
			Type:    msgType,
			Payload: msg,
		},
	})

	<-client.send // Discarding connection_info msg
	retrievedMsg := <-client.send

	require.Equal(t, msgType, retrievedMsg.Type)
	require.Equal(t, msg, []byte(retrievedMsg.Payload))
	require.False(t, retrievedMsg.RequiresWorkflowAck)
	require.Empty(t, retrievedMsg.CorrelationID)
}

func TestDispatchDirectMsg_RecipientUserIdNotFound(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)
	client := newStubClient(t, gameServer, uuid.New(), Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID:  uuid.New(),
				ConnIDs: []uuid.UUID{client.connID},
			},
		},
		Msg: &OutgoingMessage{
			WsMessage: WsMessage{
				Payload: testMsg,
			},
		},
	}

	gameServer.dispatchDirectMsg(payload)

	require.Len(t, client.send, 1) // send channel only has connection_info msg
}

func TestDispatchDirectMsg_RecipientConnIdNotFound(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)
	client := newStubClient(t, gameServer, uuid.New(), Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID:  client.userID,
				ConnIDs: []uuid.UUID{uuid.New()},
			},
		},
		Msg: &OutgoingMessage{
			WsMessage: WsMessage{
				Payload: testMsg,
			},
		},
	}

	gameServer.dispatchDirectMsg(payload)

	require.Len(t, client.send, 1) // send channel only has connection_info msg
}

func TestDispatchDirectMsg_AllUserConnections(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)

	userId := uuid.New()

	client1 := &Client{
		userID:     userId,
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan *OutgoingMessage, ClientSendChanBufSize),
	}
	client1.role.Store(Spectator)

	client2 := &Client{
		userID:     userId,
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan *OutgoingMessage, ClientSendChanBufSize),
	}
	client2.role.Store(Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client1.userID})).
		Times(1).
		Return()

	gameServer.addClient(client1)
	gameServer.addClient(client2)

	msgType := "some-msg-type"
	testMsg := []byte("test msg")

	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID: userId,
			},
		},
		Msg: &OutgoingMessage{
			WsMessage: WsMessage{
				Type:    msgType,
				Payload: testMsg,
			},
		},
	}

	gameServer.dispatchDirectMsg(payload)

	<-client1.send // Discarding connection_info msg
	retrievedMsgClient1 := <-client1.send
	<-client2.send // Discarding connection_info msg
	retrievedMsgClient2 := <-client2.send

	require.Equal(t, msgType, retrievedMsgClient1.Type)
	require.Equal(t, testMsg, []byte(retrievedMsgClient1.Payload))

	require.Equal(t, msgType, retrievedMsgClient2.Type)
	require.Equal(t, testMsg, []byte(retrievedMsgClient2.Payload))
}

func TestDispatchDirectMsg_SpecificUserConnections(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)

	userId := uuid.New()

	client1 := newStubClient(t, gameServer, userId, Player)
	client2 := newStubClient(t, gameServer, userId, Spectator)
	client3 := newStubClient(t, gameServer, userId, Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.AnyOf([]uuid.UUID{client1.userID}, []uuid.UUID{client3.userID})).
		Times(2).
		Return()

	gameServer.addClient(client1)
	gameServer.addClient(client2)
	gameServer.addClient(client3)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID:  userId,
				ConnIDs: []uuid.UUID{client2.connID, client3.connID},
			},
		},
		Msg: &OutgoingMessage{
			WsMessage: WsMessage{
				Payload: testMsg,
			},
		},
	}

	gameServer.dispatchDirectMsg(payload)

	<-client2.send // Discarding connection_info msg
	retrievedMsgClient2 := <-client2.send
	<-client3.send // Discarding connection_info msg
	retrievedMsgClient3 := <-client3.send

	require.Len(t, client1.send, 1) // send channel only has connection_info msg
	require.Equal(t, testMsg, []byte(retrievedMsgClient2.Payload))
	require.Equal(t, testMsg, []byte(retrievedMsgClient3.Payload))
}

func TestDispatchDirectMsg_SpecificUserConnections_ExcludeSpectators(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)

	userId := uuid.New()

	client1 := newStubClient(t, gameServer, userId, Player)
	client2 := newStubClient(t, gameServer, userId, Spectator)
	client3 := newStubClient(t, gameServer, userId, Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.AnyOf([]uuid.UUID{client1.userID}, []uuid.UUID{client3.userID})).
		Times(2).
		Return()

	gameServer.addClient(client1)
	gameServer.addClient(client2)
	gameServer.addClient(client3)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID:            userId,
				ConnIDs:           []uuid.UUID{client2.connID, client3.connID},
				ExcludeSpectators: true,
			},
		},
		Msg: &OutgoingMessage{
			WsMessage: WsMessage{
				Payload: testMsg,
			},
		},
	}

	gameServer.dispatchDirectMsg(payload)

	<-client3.send // Discarding connection_info msg
	retrievedMsgClient3 := <-client3.send

	require.Len(t, client1.send, 1) // send channel only has connection_info msg
	require.Len(t, client2.send, 1) // send channel only has connection_info msg
	require.Equal(t, testMsg, []byte(retrievedMsgClient3.Payload))
}

func TestDispatchDirectMsg_ExcludeSpectators(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)

	userId := uuid.New()

	client1 := newStubClient(t, gameServer, userId, Player)
	client2 := newStubClient(t, gameServer, userId, Spectator)
	client3 := newStubClient(t, gameServer, userId, Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.AnyOf([]uuid.UUID{client1.userID}, []uuid.UUID{client3.userID})).
		Times(2).
		Return()

	gameServer.addClient(client1)
	gameServer.addClient(client2)
	gameServer.addClient(client3)

	testMsg := []byte("test msg")
	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID:            userId,
				ExcludeSpectators: true,
			},
		},
		Msg: &OutgoingMessage{
			WsMessage: WsMessage{
				Payload: testMsg,
			},
		},
	}

	gameServer.dispatchDirectMsg(payload)

	<-client1.send // Discarding connection_info msg
	retrievedMsgClient1 := <-client1.send
	<-client3.send // Discarding connection_info msg
	retrievedMsgClient3 := <-client3.send

	require.Len(t, client2.send, 1) // send channel only has connection_info msg
	require.Equal(t, testMsg, []byte(retrievedMsgClient1.Payload))
	require.Equal(t, testMsg, []byte(retrievedMsgClient3.Payload))
}

func TestScheduleShutdown(t *testing.T) {
	_, gameServer := createInitializedTestGameServer(t)

	require.Nil(t, gameServer.shutdownTimer)

	gameServer.scheduleShutdown()

	require.NotNil(t, gameServer.shutdownTimer)
}

func TestShutdown(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)
	client := newStubClient(t, gameServer, uuid.New(), Player)

	mockConfig.GfClient.EXPECT().
		AddPlayers(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.gameID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	gameServer.addClient(client)

	require.False(t, gameServer.IsClosed())
	require.Len(t, gameServer.clients, 1)

	mockConfig.GfClient.EXPECT().
		RemovePlayers(gomock.Eq(gameServer.serverID), gomock.Eq([]uuid.UUID{client.userID})).
		Times(1).
		Return()

	mockConfig.GfClient.EXPECT().
		UnregisterGameServerInstance(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.instanceID)).
		Times(1).
		Return(nil)

	mockConfig.GfClient.EXPECT().
		ShutdownGameServer(gomock.Eq(gameServer.serverID)).
		Times(1).
		Return()

	mockConfig.Session.EXPECT().
		Close().
		Times(1).
		Return()

	gameServer.shutdown()

	require.True(t, gameServer.IsClosed())
	require.Len(t, gameServer.clients, 0)
	require.Equal(t, gameServer.clientCount.Load(), int32(0))
	require.Equal(t, gameServer.connCount.Load(), int32(0))
}

func TestCancelScheduledShutdown(t *testing.T) {
	_, gameServer := createInitializedTestGameServer(t)

	gameServer.scheduleShutdown()
	require.NotNil(t, gameServer.shutdownTimer)

	isCancelled := gameServer.cancelScheduledShutdown()
	require.Nil(t, gameServer.shutdownTimer)
	require.True(t, isCancelled)
}

func TestCancelScheduledShutdown_TimerAlreadyTriggered(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)

	gameServer.scheduleShutdown()
	require.NotNil(t, gameServer.shutdownTimer)

	mockConfig.Session.EXPECT().
		Close().
		Times(1).
		Return()

	mockConfig.GfClient.EXPECT().
		UnregisterGameServerInstance(gomock.Eq(gameServer.serverID), gomock.Eq(gameServer.instanceID)).
		Times(1).
		Return(nil)

	mockConfig.GfClient.EXPECT().
		ShutdownGameServer(gomock.Eq(gameServer.serverID)).
		Times(1).
		Return()

	// Trigger timer
	gameServer.shutdownTimer.Reset(0)
	time.Sleep(time.Millisecond * 50)

	isCancelled := gameServer.cancelScheduledShutdown()
	require.False(t, isCancelled)
}

func TestAckGameEvent(t *testing.T) {
	mockConfig, gameServer := createInitializedTestGameServer(t)

	correlationID := uuid.New()
	status := gameevents.AckStatusDelivered
	reason := "success"

	eventAckPayload := gameevents.EventAckPayload{
		CorrelationID: correlationID,
		InstanceID:    gameServer.instanceID,
		Status:        status,
		Reason:        reason,
	}

	mockConfig.GfClient.EXPECT().
		AcknowledgeGameEvent(gomock.Eq(gameServer.serverID), gomock.Eq(eventAckPayload)).
		Times(1).
		Return(nil)

	gameServer.ackGameEvent(correlationID, status, reason)
}

func TestBroadcast(t *testing.T) {
	_, gameServer := createInitializedTestGameServer(t)

	received := make(chan *OutgoingMessage)

	go func() {
		select {
		case msg := <-gameServer.broadcast:
			received <- msg
		case <-time.After(2 * time.Second):
			require.Fail(t, "timed out waiting for broadcast receive goroutine")
		}
	}()

	msgType := "random-msg-type"
	testMsg := []byte("test msg")

	err := gameServer.Broadcast(&WsMessage{
		Type:    msgType,
		Payload: testMsg,
	})
	require.NoError(t, err)

	select {
	case retrievedMsg := <-received:
		require.Equal(t, msgType, retrievedMsg.Type)
		require.Equal(t, testMsg, []byte(retrievedMsg.Payload))
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for broadcast")
	}
}

func TestSendDirectMsg(t *testing.T) {
	_, gameServer := createInitializedTestGameServer(t)

	received := make(chan *DirectMsgPayload)

	go func() {
		select {
		case msg := <-gameServer.directMsg:
			received <- msg
		case <-time.After(2 * time.Second):
			require.Fail(t, "timed out waiting for directMsg receive goroutine")
		}
	}()

	payload := &DirectMsgPayload{
		Recipients: []DirectMsgRecipient{
			{
				UserID: uuid.New(),
				ConnIDs: []uuid.UUID{
					uuid.New(),
				},
			},
		},
	}
	err := gameServer.SendDirectMsg(payload)
	require.NoError(t, err)

	select {
	case retrievedPayload := <-received:
		require.Equal(t, payload, retrievedPayload)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for sendDirectMsg")
	}
}

func TestRegister(t *testing.T) {
	_, gameServer := createInitializedTestGameServer(t)

	received := make(chan *Client)

	go func() {
		select {
		case client := <-gameServer.register:
			received <- client
		case <-time.After(2 * time.Second):
			require.Fail(t, "timed out waiting for register goroutine")
		}
	}()

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan *OutgoingMessage, ClientSendChanBufSize),
	}

	err := gameServer.Register(client)
	require.NoError(t, err)

	select {
	case retrievedClient := <-received:
		require.Equal(t, client, retrievedClient)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for register")
	}
}

func TestUnregister(t *testing.T) {
	_, gameServer := createInitializedTestGameServer(t)

	received := make(chan *Client)

	go func() {
		select {
		case client := <-gameServer.unregister:
			received <- client
		case <-time.After(2 * time.Second):
			require.Fail(t, "timed out waiting for unregister goroutine")
		}
	}()

	client := &Client{
		userID:     uuid.New(),
		connID:     uuid.New(),
		gameServer: gameServer,
		send:       make(chan *OutgoingMessage, ClientSendChanBufSize),
	}

	err := gameServer.Unregister(client)
	require.NoError(t, err)

	select {
	case retrievedClient := <-received:
		require.Equal(t, client, retrievedClient)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for unregister")
	}
}
