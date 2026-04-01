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

	correctGuessersResults, lastProcessedPosition := processCorrectGuessers(state, numParticipatingPlayers, guesses)
	results = append(results, correctGuessersResults...)

	nonGuessersPosition := lastProcessedPosition + 1
	nonGuessersResults := processNonGuessers(state, nonGuessersPosition)
	results = append(results, nonGuessersResults...)

	var topGuesserPoints int64 = 0
	if len(results) > 0 && len(guesses) > 0 {
		topGuesserPoints = calculatePoints(state, 1, numParticipatingPlayers, len(guesses), guesses[0].Timestamp)
	}

	artistResult := processArtist(state, topGuesserPoints, len(guesses))
	if artistResult != nil {
		results = append(results, artistResult)
	}

	updatePlayerStates(state, results)
	results = CalculateProvisionalPayouts(state, results)

	isFinalRound := int32(state.CurrentRound) >= state.NumRounds

	// TODO: roundResult should only send top 10 guessers for the round + artist in the roundResult.Results list.
	// This is to prevent this payload from ballooning as it will scale linearly with number of players
	// and we only have 4KB write limit to player wss connections.
	// Proposed impl: implement an extractTopResults helper func that takes in correctGuessersResults(already sorted),
	// artistResult, and n=10. Then return first n from correctGuessersResults and append artist at the end.

	// TODO: With the change to only send top n guessers, we can then use the updated publish game event activity to
	// send round ended message payloads to each player including the artist in one batch.

	roundResult := gameevents.RoundEndedPayload{
		Round:           state.CurrentRound,
		ArtistID:        state.CurrentArtist,
		Word:            state.CurrentWord.Text,
		DrawingDuration: state.DrawingDuration,
		Results:         results,
		TotalPot:        state.GamePot,
		IsFinalRound:    isFinalRound,
	}
	_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypeRoundEnded, roundResult, &sendGameEventOpts{
		Marker: eventbus.MarkerRoundEnded,
	})
	if err != nil {
		return fmt.Errorf("failed to send round ended event to game events stream: %w", err)
	}
	_, err = sendGameEvent[any](ctx, state, notifyCh, gameevents.TypeRoundEnded, nil, &sendGameEventOpts{
		StreamType: eventbus.DrawingUpdatesStreamType,
		Marker:     eventbus.MarkerRoundEnded,
	})
	if err != nil {
		return fmt.Errorf("failed to send round ended event to drawing updates stream: %w", err)
	}

	state.CurrentWord = types.Word{}
	state.CurrentArtist = state.NextArtist
	state.NextArtist = uuid.Nil
	state.DrawingStartedAt = time.Time{}

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
) ([]*gameevents.PlayerRoundResult, int32) {
	var position int32
	var results []*gameevents.PlayerRoundResult

	for _, guess := range guesses {
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

		var roundPoints int64 = 0
		roundStakeLost := "0"

		if tier != TierUnranked {
			roundPoints = calculatePoints(state, position, numParticipatingPlayers, len(guesses), guess.Timestamp)
		} else {
			roundStakeLost = state.StakePerRound
		}

		results = append(results, &gameevents.PlayerRoundResult{
			PlayerID:       guess.PlayerID,
			WasArtist:      false,
			GuessTimeMs:    guessTimeMs,
			Tier:           tier,
			RoundPosition:  position,
			RoundPoints:    roundPoints,
			RoundStakeLost: roundStakeLost,
			TotalPoints:    player.Points + roundPoints,
			TotalStakeLost: commonUtil.AddBigIntStrings(player.StakeLost, roundStakeLost),
		})
	}

	return results, position
}

func processNonGuessers(state *GameState, nonGuessersPosition int32) []*gameevents.PlayerRoundResult {
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

		results = append(results, &gameevents.PlayerRoundResult{
			PlayerID:       playerID,
			WasArtist:      false,
			GuessTimeMs:    -1,
			Tier:           TierUnranked,
			RoundPosition:  nonGuessersPosition,
			RoundPoints:    0,
			RoundStakeLost: state.StakePerRound,
			TotalPoints:    player.Points,
			TotalStakeLost: commonUtil.AddBigIntStrings(player.StakeLost, state.StakePerRound),
		})
	}

	return results
}

func processArtist(state *GameState, topGuesserPoints int64, totalGuesses int) *gameevents.PlayerRoundResult {
	var roundPoints int64 = 0
	roundStakeLost := "0"

	if totalGuesses > 0 {
		roundPoints = int64(float64(topGuesserPoints) * DefaultScoringConfig.ArtistRewardRatio)
	} else {
		roundStakeLost = state.StakePerRound
	}

	artist, exists := state.Players[state.CurrentArtist]
	if !exists || artist.IsEjected {
		return nil
	}

	return &gameevents.PlayerRoundResult{
		PlayerID:       artist.PlayerID,
		WasArtist:      true,
		GuessTimeMs:    -1,
		Tier:           TierArtist,
		RoundPosition:  -1,
		RoundPoints:    roundPoints,
		RoundStakeLost: roundStakeLost,
		TotalPoints:    artist.Points + roundPoints,
		TotalStakeLost: commonUtil.AddBigIntStrings(artist.StakeLost, roundStakeLost),
	}
}

func determineTier(position int32, numParticipatingPlayers int, totalGuesses int) string {
	if numParticipatingPlayers <= 0 {
		return TierUnranked
	}

	if totalGuesses == 1 {
		return DefaultScoringConfig.Tiers[0].Name
	}

	percentile := float32(position) / float32(numParticipatingPlayers)
	for _, tier := range DefaultScoringConfig.Tiers {
		if percentile <= tier.Threshold {
			return tier.Name
		}
	}

	return TierUnranked
}

func calculatePoints(
	state *GameState,
	position int32,
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

func updatePlayerStates(state *GameState, roundResults []*gameevents.PlayerRoundResult) {
	for _, result := range roundResults {
		player, exists := state.Players[result.PlayerID]
		if !exists {
			continue
		}

		player.Points = result.TotalPoints
		player.StakeLost = result.TotalStakeLost
		player.RoundsPlayed++

		if result.RoundStakeLost != "0" {
			state.GamePot = commonUtil.AddBigIntStrings(state.GamePot, result.RoundStakeLost)
		}
	}
}
