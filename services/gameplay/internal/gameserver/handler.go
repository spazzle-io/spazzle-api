package gameserver

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
)

func handleEventBusMessage(_ context.Context, _ eventbus.Message) {
	// TODO: Implement event bus message handling
	log.Info().Msg("Received eventbus message")
}

func handleClientMessage(_ *Client, _ []byte) error {
	// TODO: Implement ws client message handling
	log.Info().Msg("handled client message")

	return nil
}
