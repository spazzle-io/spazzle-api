package eventbus

import (
	"fmt"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

func streamKey(config util.Config, streamType StreamType, game GameIdentifier) string {
	return fmt.Sprintf("%s-%s:%s:%s",
		config.ServiceName,
		streamType,
		game.GameServerID.String(),
		game.GameID.String(),
	)
}
