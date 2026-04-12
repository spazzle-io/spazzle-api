package eventbus

import (
	"fmt"
	"testing"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/stretchr/testify/require"
)

func TestStreamKey(t *testing.T) {
	config := &util.Config{
		AppConfig: commonConfig.AppConfig{
			ServiceName: "gameplay",
		},
	}

	gameServerId := uuid.New()
	gameId := uuid.New()
	game := GameIdentifier{
		GameServerID: gameServerId,
		GameID:       gameId,
	}

	gotStreamKey := streamKey(config, DrawingUpdatesStreamType, game)
	expectedStreamKey := fmt.Sprintf("gameplay-drawing-updates:%s:%s", gameServerId, gameId)

	require.Equal(t, expectedStreamKey, gotStreamKey)
}

func TestMarkerKey(t *testing.T) {
	config := &util.Config{
		AppConfig: commonConfig.AppConfig{
			ServiceName: "gameplay",
		},
	}

	gameServerId := uuid.New()
	gameId := uuid.New()
	game := GameIdentifier{
		GameServerID: gameServerId,
		GameID:       gameId,
	}

	gotMarkerKey := markerKey(config, GameEventsStreamType, game, MarkerRoundEnded)
	expectedMarkerKey := fmt.Sprintf("gameplay-game-events:%s:%s:round-ended", gameServerId, gameId)

	require.Equal(t, expectedMarkerKey, gotMarkerKey)
}
