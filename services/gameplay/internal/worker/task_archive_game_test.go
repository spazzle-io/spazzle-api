package worker

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"

	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	mockstorage "github.com/spazzle-io/spazzle-api/libs/common/storage/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	mockeventbus "github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newTestPayload(t *testing.T) (*PayloadArchiveGame, *asynq.Task) {
	t.Helper()

	payload := &PayloadArchiveGame{
		ServerID:      uuid.New(),
		GameID:        uuid.New(),
		GameStake:     "2000000000000000",
		GameStartedAt: time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC),
		GameEndedAt:   time.Date(2026, time.March, 11, 12, 15, 0, 0, time.UTC),
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(TaskArchiveGame, data)
	return payload, task
}

func validGameEventStreamMessages(t *testing.T) []eventbus.Message {
	t.Helper()

	return []eventbus.Message{
		{ID: "1-0", Type: "test_event", Payload: json.RawMessage(`{"key":"value"}`)},
		{ID: "2-0", Type: "game_ended", Payload: json.RawMessage(`{
		  "results": [
			{
			  "is_evicted": false,
			  "player_id": "3b6c6a83-5995-4cfd-b59b-750893456d1d",
			  "position": 1,
			  "provisional_payout": "3000000000000000",
			  "rounds_played": 10,
			  "total_points": 20,
			  "total_stake_lost": "1000000000000000"
			},
			{
			  "is_evicted": true,
			  "player_id": "f3a2d264-7d0f-4014-81f4-129166668fc2",
			  "position": 2,
			  "provisional_payout": "0",
			  "rounds_played": 8,
			  "total_points": 0,
			  "total_stake_lost": "2000000000000000"
			}
		  ],
		  "total_pot": "4000000000000000",
		  "total_rounds": 10
		}`)},
	}
}

func TestProcessTaskArchiveGame(t *testing.T) {
	testCases := []struct {
		name          string
		input         func() (*PayloadArchiveGame, *asynq.Task)
		buildStubs    func(payload *PayloadArchiveGame, bus *mockeventbus.MockEventBus, objectStore *mockstorage.MockObjectStore, store *mockdb.MockStore)
		checkResponse func(t *testing.T, err error)
	}{
		{
			name: "success",
			input: func() (*PayloadArchiveGame, *asynq.Task) {
				return newTestPayload(t)
			},
			buildStubs: func(payload *PayloadArchiveGame, bus *mockeventbus.MockEventBus, objectStore *mockstorage.MockObjectStore, store *mockdb.MockStore) {
				game := eventbus.GameIdentifier{
					GameServerID: payload.ServerID,
					GameID:       payload.GameID,
				}

				for _, st := range eventbus.AllStreamTypes {
					bus.EXPECT().
						Replay(gomock.Any(), uuid.Nil, game, st, "0", archiveReplayLimit).
						Times(1).
						Return(eventbus.ReplayResult{
							Messages: validGameEventStreamMessages(t),
							HasMore:  false,
							LastID:   "2-0",
						}, nil)

					objectStore.EXPECT().
						Put(gomock.Any(), archiveBucket, gomock.Any(), gomock.Any(), "application/json").
						Times(1).
						Return(nil)
				}

				store.EXPECT().
					ArchiveGameTx(gomock.Any(), gomock.Eq(db.ArchiveGameTxParams{
						GameID:    payload.GameID,
						ServerID:  payload.ServerID,
						NumRounds: int32(10),
						TotalPot:  big.NewInt(int64(4000000000000000)),
						GameStake: big.NewInt(int64(2000000000000000)),
						PlayerResults: []db.GamePlayerResult{
							{
								UserID:            uuid.MustParse("3b6c6a83-5995-4cfd-b59b-750893456d1d"),
								Score:             int32(20),
								Pnl:               big.NewInt(int64(2000000000000000)),
								Position:          int32(1),
								RoundsPlayed:      int32(10),
								ProvisionalPayout: big.NewInt(int64(3000000000000000)),
								TotalStakeLost:    big.NewInt(int64(1000000000000000)),
								IsEvicted:         false,
							},
							{
								UserID:            uuid.MustParse("f3a2d264-7d0f-4014-81f4-129166668fc2"),
								Score:             int32(0),
								Pnl:               big.NewInt(int64(-2000000000000000)),
								Position:          int32(2),
								RoundsPlayed:      int32(8),
								ProvisionalPayout: big.NewInt(int64(0)),
								TotalStakeLost:    big.NewInt(int64(2000000000000000)),
								IsEvicted:         true,
							},
						},
						StartedAt: time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC),
						EndedAt:   time.Date(2026, time.March, 11, 12, 15, 0, 0, time.UTC),
					})).
					Times(1).
					Return(db.ArchiveGameTxResult{}, nil)

				bus.EXPECT().
					Cleanup(gomock.Any(), game).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "invalid payload",
			input: func() (*PayloadArchiveGame, *asynq.Task) {
				return &PayloadArchiveGame{}, asynq.NewTask(TaskArchiveGame, []byte("invalid json"))
			},
			buildStubs: func(payload *PayloadArchiveGame, bus *mockeventbus.MockEventBus, objectStore *mockstorage.MockObjectStore, store *mockdb.MockStore) {
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name: "could not replay game stream",
			input: func() (*PayloadArchiveGame, *asynq.Task) {
				return newTestPayload(t)
			},
			buildStubs: func(payload *PayloadArchiveGame, bus *mockeventbus.MockEventBus, objectStore *mockstorage.MockObjectStore, store *mockdb.MockStore) {
				game := eventbus.GameIdentifier{
					GameServerID: payload.ServerID,
					GameID:       payload.GameID,
				}

				bus.EXPECT().
					Replay(gomock.Any(), uuid.Nil, game, gomock.Any(), "0", archiveReplayLimit).
					AnyTimes().
					Return(eventbus.ReplayResult{}, errors.New("failed to replay game stream"))
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name: "no messages to archive in s3",
			input: func() (*PayloadArchiveGame, *asynq.Task) {
				return newTestPayload(t)
			},
			buildStubs: func(payload *PayloadArchiveGame, bus *mockeventbus.MockEventBus, objectStore *mockstorage.MockObjectStore, store *mockdb.MockStore) {
				game := eventbus.GameIdentifier{
					GameServerID: payload.ServerID,
					GameID:       payload.GameID,
				}

				for _, st := range eventbus.AllStreamTypes {
					bus.EXPECT().
						Replay(gomock.Any(), uuid.Nil, game, st, "0", archiveReplayLimit).
						Times(1).
						Return(eventbus.ReplayResult{
							Messages: []eventbus.Message{},
							HasMore:  false,
							LastID:   "2-0",
						}, nil)
				}

				bus.EXPECT().
					Cleanup(gomock.Any(), game).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "could not archive game in s3",
			input: func() (*PayloadArchiveGame, *asynq.Task) {
				return newTestPayload(t)
			},
			buildStubs: func(payload *PayloadArchiveGame, bus *mockeventbus.MockEventBus, objectStore *mockstorage.MockObjectStore, store *mockdb.MockStore) {
				game := eventbus.GameIdentifier{
					GameServerID: payload.ServerID,
					GameID:       payload.GameID,
				}

				bus.EXPECT().
					Replay(gomock.Any(), uuid.Nil, game, gomock.Any(), "0", archiveReplayLimit).
					AnyTimes().
					Return(eventbus.ReplayResult{
						Messages: validGameEventStreamMessages(t),
						HasMore:  false,
						LastID:   "2-0",
					}, nil)

				objectStore.EXPECT().
					Put(gomock.Any(), archiveBucket, gomock.Any(), gomock.Any(), "application/json").
					AnyTimes().
					Return(errors.New("failed to archive game in s3"))
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name: "game events stream lacks game ended payload",
			input: func() (*PayloadArchiveGame, *asynq.Task) {
				return newTestPayload(t)
			},
			buildStubs: func(payload *PayloadArchiveGame, bus *mockeventbus.MockEventBus, objectStore *mockstorage.MockObjectStore, store *mockdb.MockStore) {
				game := eventbus.GameIdentifier{
					GameServerID: payload.ServerID,
					GameID:       payload.GameID,
				}

				for _, st := range eventbus.AllStreamTypes {
					bus.EXPECT().
						Replay(gomock.Any(), uuid.Nil, game, st, "0", archiveReplayLimit).
						Times(1).
						Return(eventbus.ReplayResult{
							Messages: []eventbus.Message{
								{ID: "1-0", Type: "test_event", Payload: json.RawMessage(`{"key":"value"}`)},
							},
							HasMore: false,
							LastID:  "2-0",
						}, nil)

					objectStore.EXPECT().
						Put(gomock.Any(), archiveBucket, gomock.Any(), gomock.Any(), "application/json").
						Times(1).
						Return(nil)
				}
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name: "game record already exists in db",
			input: func() (*PayloadArchiveGame, *asynq.Task) {
				return newTestPayload(t)
			},
			buildStubs: func(payload *PayloadArchiveGame, bus *mockeventbus.MockEventBus, objectStore *mockstorage.MockObjectStore, store *mockdb.MockStore) {
				game := eventbus.GameIdentifier{
					GameServerID: payload.ServerID,
					GameID:       payload.GameID,
				}

				for _, st := range eventbus.AllStreamTypes {
					bus.EXPECT().
						Replay(gomock.Any(), uuid.Nil, game, st, "0", archiveReplayLimit).
						Times(1).
						Return(eventbus.ReplayResult{
							Messages: validGameEventStreamMessages(t),
							HasMore:  false,
							LastID:   "2-0",
						}, nil)

					objectStore.EXPECT().
						Put(gomock.Any(), archiveBucket, gomock.Any(), gomock.Any(), "application/json").
						Times(1).
						Return(nil)
				}

				store.EXPECT().
					ArchiveGameTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ArchiveGameTxResult{}, db.ErrGameAlreadyExists)

				bus.EXPECT().
					Cleanup(gomock.Any(), game).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "could not archive game in db",
			input: func() (*PayloadArchiveGame, *asynq.Task) {
				return newTestPayload(t)
			},
			buildStubs: func(payload *PayloadArchiveGame, bus *mockeventbus.MockEventBus, objectStore *mockstorage.MockObjectStore, store *mockdb.MockStore) {
				game := eventbus.GameIdentifier{
					GameServerID: payload.ServerID,
					GameID:       payload.GameID,
				}

				for _, st := range eventbus.AllStreamTypes {
					bus.EXPECT().
						Replay(gomock.Any(), uuid.Nil, game, st, "0", archiveReplayLimit).
						Times(1).
						Return(eventbus.ReplayResult{
							Messages: validGameEventStreamMessages(t),
							HasMore:  false,
							LastID:   "2-0",
						}, nil)

					objectStore.EXPECT().
						Put(gomock.Any(), archiveBucket, gomock.Any(), gomock.Any(), "application/json").
						Times(1).
						Return(nil)
				}

				store.EXPECT().
					ArchiveGameTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ArchiveGameTxResult{}, errors.New("failed to archive game in db"))
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
		{
			name: "could not cleanup game in eventbus",
			input: func() (*PayloadArchiveGame, *asynq.Task) {
				return newTestPayload(t)
			},
			buildStubs: func(payload *PayloadArchiveGame, bus *mockeventbus.MockEventBus, objectStore *mockstorage.MockObjectStore, store *mockdb.MockStore) {
				game := eventbus.GameIdentifier{
					GameServerID: payload.ServerID,
					GameID:       payload.GameID,
				}

				for _, st := range eventbus.AllStreamTypes {
					bus.EXPECT().
						Replay(gomock.Any(), uuid.Nil, game, st, "0", archiveReplayLimit).
						Times(1).
						Return(eventbus.ReplayResult{
							Messages: validGameEventStreamMessages(t),
							HasMore:  false,
							LastID:   "2-0",
						}, nil)

					objectStore.EXPECT().
						Put(gomock.Any(), archiveBucket, gomock.Any(), gomock.Any(), "application/json").
						Times(1).
						Return(nil)
				}

				store.EXPECT().
					ArchiveGameTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ArchiveGameTxResult{}, nil)

				bus.EXPECT().
					Cleanup(gomock.Any(), game).
					Times(1).
					Return(errors.New("failed to cleanup game in eventbus"))
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, asynq.SkipRetry)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			bus := mockeventbus.NewMockEventBus(ctrl)
			store := mockdb.NewMockStore(ctrl)
			objectStore := mockstorage.NewMockObjectStore(ctrl)

			processor := &RedisTaskProcessor{
				bus:         bus,
				store:       store,
				objectStore: objectStore,
			}

			payload, task := tc.input()

			tc.buildStubs(payload, bus, objectStore, store)

			err := processor.ProcessTaskArchiveGame(context.Background(), task)
			tc.checkResponse(t, err)
		})
	}
}
