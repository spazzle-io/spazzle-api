package eventbus

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/stretchr/testify/require"
)

func TestStreamKey(t *testing.T) {
	config := util.Config{
		ServiceName: "gameplay",
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
