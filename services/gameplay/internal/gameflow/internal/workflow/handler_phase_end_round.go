package workflow

import (
	"fmt"
	"sort"
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"

	"github.com/google/uuid"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.temporal.io/sdk/workflow"
)

const endRoundCooldown = 8 * time.Second

func handlePhaseEndRound(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) {
	state.Logger().Info("entering end-round phase")

	for {
		err := endRound(ctx, state, notifyCh)
		if err == nil {
			break
		}

		state.Logger().Warn("error occurred in the end-round phase", "error", err)

		if err = workflow.Sleep(ctx, phaseCooldownDuration); err != nil {
			state.Logger().Warn("failed to cooldown after end-round phase attempt", "error", err)
		}
	}
}

func endRound(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) error {
	results := make([]*gameevents.PlayerRoundResult, 0)

	guesses := getSortedGuesses(state)
	numParticipatingPlayers := countParticipatingPlayers(state)

	correctGuessersResults, lastProcessedPosition, err := processCorrectGuessers(state, numParticipatingPlayers, guesses)
	if err != nil {
		state.Phase = types.PhaseEndGame
		return fmt.Errorf("could not determine correct guessers' results: %v", err)
	}

	results = append(results, correctGuessersResults...)

	nonGuessersPosition := lastProcessedPosition + 1
	nonGuessersResults, err := processNonGuessers(state, nonGuessersPosition)
	if err != nil {
		state.Phase = types.PhaseEndGame
		return fmt.Errorf("could not determine non guessers' results: %v", err)
	}

	results = append(results, nonGuessersResults...)

	var topGuesserPoints int64 = 0
	if len(results) > 0 && len(guesses) > 0 {
		topGuesserPoints = calculatePoints(state, 1, numParticipatingPlayers, len(guesses), guesses[0].Timestamp)
	}

	artistResult, err := processArtist(state, topGuesserPoints, len(guesses))
	if err != nil {
		state.Phase = types.PhaseEndGame
		return fmt.Errorf("could not determine artist's results: %v", err)
	}

	if artistResult != nil {
		results = append(results, artistResult)
	}

	err = updatePlayerStates(state, results)
	if err != nil {
		state.Phase = types.PhaseEndGame
		return fmt.Errorf("could not update player states: %v", err)
	}

	results, err = CalculateProvisionalPayouts(state, results)
	if err != nil {
		state.Phase = types.PhaseEndGame
		return fmt.Errorf("could not calculate provisional payouts: %v", err)
	}

	events := make([]GameEvent[*gameevents.PlayerRoundResult], len(results))
	for i, r := range results {
		events[i] = GameEvent[*gameevents.PlayerRoundResult]{
			TargetClientID: r.PlayerID,
			Payload:        r,
		}
	}
	err = sendGameEvents(ctx, state, gameevents.TypePlayerRoundResult, events)
	if err != nil {
		return fmt.Errorf("failed to send player round results: %w", err)
	}

	isFinalRound := state.CurrentRound >= state.NumRounds
	broadcastedResults := topRoundResults(correctGuessersResults, artistResult, 10)

	roundResults := gameevents.RoundEndedPayload{
		Round:           state.CurrentRound,
		ArtistID:        state.CurrentArtist,
		Word:            state.CurrentWord.Text,
		DrawingDuration: state.DrawingDuration,
		Results:         broadcastedResults,
		TotalPot:        state.GamePot,
		IsFinalRound:    isFinalRound,
	}
	_, err = sendGameEvent(ctx, state, notifyCh, gameevents.TypeRoundEnded, roundResults,
		WithMarker(eventbus.Marker{
			Round: state.CurrentRound,
			Type:  eventbus.MarkerRoundEnded,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to send round ended event to game events stream: %w", err)
	}

	markerPayload := gameevents.RoundMarkerPayload{
		Round: state.CurrentRound,
	}
	_, err = sendGameEvent[any](ctx, state, notifyCh, gameevents.TypeRoundEnded, markerPayload,
		WithStreamType(eventbus.DrawingUpdatesStreamType),
		WithMarker(eventbus.Marker{
			Round: state.CurrentRound,
			Type:  eventbus.MarkerRoundEnded,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to send round ended event to drawing updates stream: %w", err)
	}

	state.CurrentWord = types.Word{}
	state.CurrentArtist = state.NextArtist
	state.NextArtist = uuid.Nil
	state.DrawingStartedAt = time.Time{}
	delete(state.CorrectGuesses, state.CurrentRound)
	delete(state.CorrectGuessers, state.CurrentRound)

	err = workflow.Sleep(ctx, endRoundCooldown)
	if err != nil {
		state.Logger().Error("failed to sleep after sending end round event", "error", err)
	}

	if isFinalRound || state.IsTerminated {
		state.Phase = types.PhaseEndGame
	} else {
		state.CurrentRound++
		state.Phase = types.PhasePrepareRound
	}

	state.Logger().Info("round ended", "round", state.CurrentRound)
	return nil
}

func getSortedGuesses(state *GameState) []types.CorrectGuess {
	guesses := make([]types.CorrectGuess, len(state.CorrectGuesses[state.CurrentRound]))
	copy(guesses, state.CorrectGuesses[state.CurrentRound])

	sort.Slice(guesses, func(i, j int) bool {
		return guesses[i].Timestamp.Before(guesses[j].Timestamp)
	})

	return guesses
}

func countParticipatingPlayers(state *GameState) int {
	var participatingPlayers int
	for _, playerID := range sortedUUIDs(state.Players) {
		player := state.Players[playerID]
		if !player.IsEjected && player.PlayerID != state.CurrentArtist {
			participatingPlayers++
		}
	}

	return participatingPlayers
}

func processCorrectGuessers(
	state *GameState,
	numParticipatingPlayers int,
	guesses []types.CorrectGuess,
) ([]*gameevents.PlayerRoundResult, int, error) {
	var position int
	var results []*gameevents.PlayerRoundResult

	for _, guess := range guesses {
		var err error
		var roundPoints int64 = 0
		roundStakeLost := commonUtil.ZeroWei()

		player, exists := state.Players[guess.PlayerID]
		if !exists || player.IsEjected {
			continue
		}

		if player.PlayerID == state.CurrentArtist {
			continue
		}

		position++
		tier := determineTier(position, numParticipatingPlayers, len(guesses))
		guessTimeMs := guess.Timestamp.Sub(state.DrawingStartedAt).Milliseconds()

		if tier != TierUnranked {
			roundPoints = calculatePoints(state, position, numParticipatingPlayers, len(guesses), guess.Timestamp)
		} else {
			roundStakeLost, err = commonUtil.NewNonNegativeWei(state.StakePerRound)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to parse round stake lost: %w", err)
			}
		}

		playerStakeLost, err := commonUtil.NewNonNegativeWei(player.StakeLost)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse player stake lost: %w", err)
		}

		results = append(results, &gameevents.PlayerRoundResult{
			PlayerID:       guess.PlayerID,
			WasArtist:      false,
			GuessTimeMs:    guessTimeMs,
			Tier:           tier,
			RoundPosition:  position,
			RoundPoints:    roundPoints,
			RoundStakeLost: roundStakeLost.String(),
			TotalPoints:    player.Points + roundPoints,
			TotalStakeLost: playerStakeLost.Add(roundStakeLost).String(),
		})
	}

	return results, position, nil
}

func processNonGuessers(state *GameState, nonGuessersPosition int) ([]*gameevents.PlayerRoundResult, error) {
	var results []*gameevents.PlayerRoundResult
	for _, playerID := range sortedUUIDs(state.Players) {
		player := state.Players[playerID]

		if player.IsEjected {
			continue
		}

		if playerID == state.CurrentArtist {
			continue
		}

		guessedCorrectly := state.CorrectGuessers[state.CurrentRound][playerID]
		if guessedCorrectly {
			continue
		}

		stakeLost, err := commonUtil.NewNonNegativeWei(player.StakeLost)
		if err != nil {
			return nil, fmt.Errorf("failed to parse player stake lost: %w", err)
		}

		stakePerRound, err := commonUtil.NewNonNegativeWei(state.StakePerRound)
		if err != nil {
			return nil, fmt.Errorf("failed to parse stake per round: %w", err)
		}

		totalStakeLost := stakeLost.Add(stakePerRound)

		results = append(results, &gameevents.PlayerRoundResult{
			PlayerID:       playerID,
			WasArtist:      false,
			GuessTimeMs:    -1,
			Tier:           TierUnranked,
			RoundPosition:  nonGuessersPosition,
			RoundPoints:    0,
			RoundStakeLost: state.StakePerRound,
			TotalPoints:    player.Points,
			TotalStakeLost: totalStakeLost.String(),
		})
	}

	return results, nil
}

func processArtist(state *GameState, topGuesserPoints int64, totalGuesses int) (*gameevents.PlayerRoundResult, error) {
	var err error
	var roundPoints int64 = 0
	roundStakeLost := commonUtil.ZeroWei()

	artist, exists := state.Players[state.CurrentArtist]
	if !exists || artist.IsEjected {
		return nil, nil
	}

	if totalGuesses > 0 {
		roundPoints = (topGuesserPoints * DefaultScoringConfig.ArtistRewardPct) / 100
	} else {
		roundStakeLost, err = commonUtil.NewNonNegativeWei(state.StakePerRound)
		if err != nil {
			return nil, fmt.Errorf("failed to parse round stake lost: %w", err)
		}
	}

	stakeLost, err := commonUtil.NewNonNegativeWei(artist.StakeLost)
	if err != nil {
		return nil, fmt.Errorf("failed to parse artist stake lost: %w", err)
	}

	totalStakeLost := stakeLost.Add(roundStakeLost)

	return &gameevents.PlayerRoundResult{
		PlayerID:       artist.PlayerID,
		WasArtist:      true,
		GuessTimeMs:    -1,
		Tier:           TierArtist,
		RoundPosition:  -1,
		RoundPoints:    roundPoints,
		RoundStakeLost: roundStakeLost.String(),
		TotalPoints:    artist.Points + roundPoints,
		TotalStakeLost: totalStakeLost.String(),
	}, nil
}

func determineTier(position int, numParticipatingPlayers int, totalGuesses int) string {
	if numParticipatingPlayers <= 0 {
		return TierUnranked
	}

	if totalGuesses == 1 {
		return DefaultScoringConfig.Tiers[0].Name
	}

	scaledPosition := position * 100
	for _, tier := range DefaultScoringConfig.Tiers {
		scaledThreshold := tier.Threshold * numParticipatingPlayers
		if scaledPosition <= scaledThreshold {
			return tier.Name
		}
	}

	return TierUnranked
}

func calculatePoints(
	state *GameState,
	position int,
	numParticipatingPlayers int,
	totalGuesses int,
	guessTime time.Time,
) int64 {
	if state.DrawingDuration <= 0 {
		return 0
	}

	tier := determineTier(position, numParticipatingPlayers, totalGuesses)

	var basePoints int16 = 0
	for _, t := range DefaultScoringConfig.Tiers {
		if t.Name == tier {
			basePoints = t.BasePoints
			break
		}
	}

	if basePoints == 0 {
		return 0
	}

	guessDuration := guessTime.Sub(state.DrawingStartedAt)
	speedFactor := 1.0 - (float64(guessDuration) / float64(state.DrawingDuration))
	if speedFactor < 0 {
		speedFactor = 0
	}

	return int64(float64(basePoints) * speedFactor)
}

func updatePlayerStates(state *GameState, roundResults []*gameevents.PlayerRoundResult) error {
	for _, result := range roundResults {
		player, exists := state.Players[result.PlayerID]
		if !exists {
			continue
		}

		player.Points = result.TotalPoints
		player.StakeLost = result.TotalStakeLost
		player.RoundsPlayed++

		gamePot, err := commonUtil.NewNonNegativeWei(state.GamePot)
		if err != nil {
			return fmt.Errorf("failed to parse game pot: %w", err)
		}

		roundStakeLost, err := commonUtil.NewNonNegativeWei(result.RoundStakeLost)
		if err != nil {
			return fmt.Errorf("failed to parse round stake lost: %w", err)
		}

		updatedGamePot := gamePot.Add(roundStakeLost)
		state.GamePot = updatedGamePot.String()
	}

	return nil
}

func topRoundResults(
	correctGuessers []*gameevents.PlayerRoundResult,
	artist *gameevents.PlayerRoundResult,
	n int,
) []*gameevents.PlayerRoundResult {
	n = min(n, len(correctGuessers))

	top := correctGuessers[:n:n]
	if artist == nil {
		return top
	}

	return append(top, artist)
}
