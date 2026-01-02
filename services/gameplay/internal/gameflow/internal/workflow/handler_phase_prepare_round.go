package workflow

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

func handlePhasePrepareRound(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info("entering prepare round phase")

	for {
		err := prepareRound(ctx, state, notifyCh, logger)
		if err != nil {
			logger.Warn("failed to prepare round", "error", err)
		}

		if state.Phase != types.PhasePrepareRound {
			return
		}
	}
}

func prepareRound(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) (err error) {
	if !hasEnoughPlayers(state) {
		state.Phase = types.PhaseWaiting
		return errors.New("not enough players. returning to waiting phase")
	}

	artistID := state.CurrentArtist
	if artistID == uuid.Nil {
		artistID, err = selectArtist(ctx, state, logger)
		if err != nil {
			return fmt.Errorf("failed to select artist: %w", err)
		}
	}

	artistSelectedPayload, err := json.Marshal(&gameevents.ArtistSelectedPayload{
		ArtistID:    artistID,
		RoundNumber: state.RoundNumber,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal artist selected payload: %w", err)
	}

	eventDelivered, err := sendGameEvent(
		ctx, state, notifyCh, gameevents.TypeArtistSelected, artistSelectedPayload, artistID, true,
	)
	if err != nil {
		return fmt.Errorf("failed to send artist selected event: %w", err)
	}

	if !eventDelivered {
		if state.CurrentArtist != uuid.Nil {
			// TODO: Fine state.CurrentArtist
			state.CurrentArtist = uuid.Nil
			return errors.New("previously selected artist left the game")
		}

		return errors.New("failed to deliver artist selected event")
	}

	state.CurrentArtist = artistID

	if !hasEnoughPlayers(state) {
		state.Phase = types.PhaseWaiting
		return errors.New("not enough players. returning to waiting phase")
	}

	state.Phase = types.PhaseInRound

	logger.Info("selected artist", "artist_id", artistID, "round_number", state.RoundNumber)

	return nil
}
