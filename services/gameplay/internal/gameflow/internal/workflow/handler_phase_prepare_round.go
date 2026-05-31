package workflow

import (
	"errors"
	"fmt"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.temporal.io/sdk/workflow"
)

func handlePhasePrepareRound(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) {
	state.Logger().Info("entering prepare round phase")

	for {
		err := prepareRound(ctx, state, notifyCh)
		if err != nil {
			state.Logger().Warn("failed to prepare round", "error", err)

			if state.CurrentRound > DefaultRoundNumber {
				state.Logger().Info("skipping round due to prepare round failure")
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
			state.Logger().Warn("failed to cooldown after prepare round phase attempt", "error", err)
		}
	}
}

func prepareRound(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) (err error) {
	if !hasEnoughPlayers(state) {
		state.Phase = types.PhaseWaiting
		return errors.New("not enough players. returning to waiting phase")
	}

	artistID, err := selectAndNotifyArtist(ctx, state, notifyCh)
	if err != nil || artistID == uuid.Nil {
		return fmt.Errorf("failed to select and notify artist: %w", err)
	}

	if !hasEnoughPlayers(state) {
		state.Phase = types.PhaseWaiting
		return errors.New("not enough players. returning to waiting phase")
	}

	state.CurrentArtist = artistID
	state.Phase = types.PhaseInRound

	state.Logger().Info("selected artist", "artist_id", artistID)
	return nil
}

func selectAndNotifyArtist(
	ctx workflow.Context,
	state *GameState,
	notifyCh workflow.Channel,
) (artistID uuid.UUID, err error) {
	for {
		artistID = state.CurrentArtist
		if artistID == uuid.Nil {
			artistID, err = selectArtist(ctx, state)
			if err != nil {
				return uuid.Nil, fmt.Errorf("failed to select artist: %w", err)
			}
		}

		payload := gameevents.ArtistSelectedPayload{
			ArtistID:     artistID,
			CurrentRound: state.CurrentRound,
		}
		delivered, err := sendGameEvent(
			ctx, state, notifyCh, gameevents.TypeArtistSelected, payload,
			WithTargetClient(artistID),
			WithAck(),
		)
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to send artist selected event: %w", err)
		}

		if delivered {
			state.Logger().Info("artist selected and notified", "artist_id", artistID)
			return artistID, nil
		}

		if state.CurrentArtist != uuid.Nil {
			// TODO: Fine state.CurrentArtist
			state.CurrentArtist = uuid.Nil
			state.Logger().Warn("previously selected artist fined for leaving the game", "artist_id", artistID)
		}

		if artist, exists := state.Players[artistID]; exists {
			artist.IsConnected = false
		}
	}
}

func publishRoundStartedEvent(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) error {
	payload := gameevents.RoundMarkerPayload{
		Round: state.CurrentRound,
	}

	_, err := sendGameEvent[any](ctx, state, notifyCh, gameevents.TypeRoundStarted, payload,
		WithMarker(eventbus.Marker{
			Round: state.CurrentRound,
			Type:  eventbus.MarkerRoundStarted,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to send round started event to game events stream: %w", err)
	}

	_, err = sendGameEvent[any](ctx, state, notifyCh, gameevents.TypeRoundStarted, payload,
		WithStreamType(eventbus.DrawingUpdatesStreamType),
		WithMarker(eventbus.Marker{
			Round: state.CurrentRound,
			Type:  eventbus.MarkerRoundStarted,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to send round started event to drawing updates stream: %w", err)
	}

	return nil
}
