package workflow

import (
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

			if state.CurrentRound > DefaultRoundNumber {
				logger.Info("ending game due to prepare round failure")
				state.Phase = types.PhaseEndRound
				return
			}
		}

		if state.IsTerminated {
			state.Phase = types.PhaseEndRound
			return
		}

		if state.Phase != types.PhasePrepareRound {
			return
		}

		if err = workflow.Sleep(ctx, phaseCooldownDuration); err != nil {
			logger.Warn("failed to cooldown after prepare round phase attempt", "error", err)
		}
	}
}

func prepareRound(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) (err error) {
	if !hasEnoughPlayers(state) {
		state.Phase = types.PhaseWaiting
		return errors.New("not enough players. returning to waiting phase")
	}

	artistID, err := selectAndNotifyArtist(ctx, state, notifyCh, logger)
	if err != nil || artistID == uuid.Nil {
		return fmt.Errorf("failed to select and notify artist: %w", err)
	}

	if !hasEnoughPlayers(state) {
		state.Phase = types.PhaseWaiting
		return errors.New("not enough players. returning to waiting phase")
	}

	state.CurrentArtist = artistID
	state.Phase = types.PhaseInRound

	logger.Info("selected artist", "artist_id", artistID)
	return nil
}

func selectAndNotifyArtist(
	ctx workflow.Context,
	state *GameState,
	notifyCh workflow.Channel,
	logger log.Logger,
) (artistID uuid.UUID, err error) {
	for {
		artistID = state.CurrentArtist
		if artistID == uuid.Nil {
			artistID, err = selectArtist(ctx, state, logger)
			if err != nil {
				return uuid.Nil, fmt.Errorf("failed to select artist: %w", err)
			}
		}

		payload := gameevents.ArtistSelectedPayload{
			ArtistID:     artistID,
			CurrentRound: state.CurrentRound,
		}
		delivered, err := sendGameEvent(
			ctx, state, notifyCh, gameevents.TypeArtistSelected, payload, &sendGameEventOpts{
				TargetClientID: artistID,
				WaitForAck:     true,
			},
		)
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to send artist selected event: %w", err)
		}

		if delivered {
			logger.Info("artist selected and notified", "artist_id", artistID)
			return artistID, nil
		}

		if state.CurrentArtist != uuid.Nil {
			// TODO: Fine state.CurrentArtist
			state.CurrentArtist = uuid.Nil
			logger.Warn("previously selected artist fined for leaving the game", "artist_id", artistID)
		}

		if artist, exists := state.Players[artistID]; exists {
			artist.IsConnected = false
		}
	}
}
