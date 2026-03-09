package worker

import (
	"context"
	"encoding/json"
	"testing"

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
		GameServerID: uuid.New(),
		GameID:       uuid.New(),
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(TaskArchiveGame, data)
	return payload, task
}

func TestProcessTaskArchiveGame_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := mockeventbus.NewMockEventBus(ctrl)
	mockObjectStore := mockstorage.NewMockObjectStore(ctrl)

	processor := &RedisTaskProcessor{
		bus:         mockBus,
		objectStore: mockObjectStore,
	}

	payload, task := newTestPayload(t)
	game := eventbus.GameIdentifier{
		GameServerID: payload.GameServerID,
		GameID:       payload.GameID,
	}

	messages := []eventbus.Message{
		{ID: "1-0", Type: "test_event", Payload: json.RawMessage(`{"key":"value"}`)},
		{ID: "2-0", Type: "test_event", Payload: json.RawMessage(`{"key":"value2"}`)},
	}

	for _, st := range eventbus.AllStreamTypes {
		mockBus.EXPECT().
			Replay(gomock.Any(), uuid.Nil, game, st, "0", archiveReplayLimit).
			Return(eventbus.ReplayResult{
				Messages: messages,
				HasMore:  false,
				LastID:   "2-0",
			}, nil)

		mockObjectStore.EXPECT().
			Put(gomock.Any(), archiveBucket, gomock.Any(), gomock.Any(), "application/json").
			Return(nil)
	}

	mockBus.EXPECT().
		Cleanup(gomock.Any(), game).
		Return(nil)

	err := processor.ProcessTaskArchiveGame(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskArchiveGame_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBus := mockeventbus.NewMockEventBus(ctrl)
	mockObjectStore := mockstorage.NewMockObjectStore(ctrl)

	processor := &RedisTaskProcessor{
		bus:         mockBus,
		objectStore: mockObjectStore,
	}

	task := asynq.NewTask(TaskArchiveGame, []byte("invalid json"))

	err := processor.ProcessTaskArchiveGame(context.Background(), task)
	require.Error(t, err)
	require.ErrorIs(t, err, asynq.SkipRetry)
}
