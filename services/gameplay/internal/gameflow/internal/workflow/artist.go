package workflow

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"go.temporal.io/sdk/log"

	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
)

func getEligibleArtists(state *GameState) []uuid.UUID {
	eligibleArtists := make([]uuid.UUID, 0)
	for _, playerID := range sortedUUIDs(state.Players) {
		player := state.Players[playerID]

		if !player.IsConnected || player.IsEjected {
			continue
		}

		if state.PastArtists[playerID] {
			continue
		}

		eligibleArtists = append(eligibleArtists, playerID)
	}

	return eligibleArtists
}

func selectArtist(ctx workflow.Context, state *GameState, logger log.Logger) (uuid.UUID, error) {
	eligibleArtists := getEligibleArtists(state)

	if len(eligibleArtists) == 0 {
		state.PastArtists = make(map[uuid.UUID]bool)
		eligibleArtists = getEligibleArtists(state)
	}

	if len(eligibleArtists) == 0 {
		return uuid.Nil, errors.New("no eligible artists found")
	}

	encodedArtistID := workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		maxVal := big.NewInt(int64(len(eligibleArtists)))
		n, err := rand.Int(rand.Reader, maxVal)
		if err != nil {
			logger.Warn("failed to generate random artist ID", "err", err)
			return eligibleArtists[0]
		}
		return eligibleArtists[n.Int64()]
	})

	var artistID uuid.UUID
	err := encodedArtistID.Get(&artistID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to get artist: %w", err)
	}

	state.PastArtists[artistID] = true

	return artistID, nil
}
