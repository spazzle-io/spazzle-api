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

	marker := Marker{Type: MarkerRoundEnded, Round: 1}
	gotMarkerKey := markerKey(config, GameEventsStreamType, game, marker)
	expectedMarkerKey := fmt.Sprintf("gameplay-game-events:%s:%s:round-ended:1", gameServerId, gameId)

	require.Equal(t, expectedMarkerKey, gotMarkerKey)
}

func TestMarkerKey_RoundScoped(t *testing.T) {
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

	marker1 := Marker{Type: MarkerRoundEnded, Round: 1}
	marker2 := Marker{Type: MarkerRoundEnded, Round: 2}

	key1 := markerKey(config, GameEventsStreamType, game, marker1)
	key2 := markerKey(config, GameEventsStreamType, game, marker2)

	require.NotEqual(t, key1, key2)
	require.Contains(t, key1, "round-ended:1")
	require.Contains(t, key2, "round-ended:2")
}

func TestMarkerRegistryKey(t *testing.T) {
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

	gotKey := markerRegistryKey(config, GameEventsStreamType, game)
	expectedKey := fmt.Sprintf("gameplay-game-events:%s:%s:marker-registry", gameServerId, gameId)

	require.Equal(t, expectedKey, gotKey)
}

func TestMarker_String(t *testing.T) {
	require.Equal(t, "round-started:1", Marker{Type: MarkerRoundStarted, Round: 1}.String())
	require.Equal(t, "round-ended:3", Marker{Type: MarkerRoundEnded, Round: 3}.String())
	require.Equal(t, "round-started:10", Marker{Type: MarkerRoundStarted, Round: 10}.String())
}
