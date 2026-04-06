package workflow

import (
	"fmt"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
)

const (
	TierArtist   = "artist"
	TierUnranked = "unranked"
)

type TierConfig struct {
	Name       string
	Threshold  int
	BasePoints int16
}

var DefaultTierConfigs = []TierConfig{
	{Name: "top_20", Threshold: 20, BasePoints: 100},
	{Name: "top_50", Threshold: 50, BasePoints: 50},
	{Name: "top_80", Threshold: 80, BasePoints: 25},
}

type ScoringConfig struct {
	Tiers []TierConfig
	// Percentage of top guesser's points to be awarded to the artist
	ArtistRewardPct int64
	// Percentage of the game pot to be paid out to players
	PayoutPct int64
}

var DefaultScoringConfig = ScoringConfig{
	Tiers:           DefaultTierConfigs,
	ArtistRewardPct: 50,
	PayoutPct:       90,
}

type PointsHolder interface {
	GetPoints() int64
	SetProvisionalPayout(payout commonUtil.Wei)
}

func CalculateProvisionalPayouts[T PointsHolder](state *GameState, holders []T) ([]T, error) {
	var totalPoints int64
	for _, playerID := range sortedUUIDs(state.Players) {
		player := state.Players[playerID]
		totalPoints += player.Points
	}

	gamePot, err := commonUtil.NewNonNegativeWei(state.GamePot)
	if err != nil {
		return nil, fmt.Errorf("game pot must be non negative: %v", err)
	}

	payoutPool, err := gamePot.Mul(DefaultScoringConfig.PayoutPct).Div(100)
	if err != nil {
		return nil, fmt.Errorf("could not determine payout pool: %v", err)
	}

	for i := range holders {
		if holders[i].GetPoints() == 0 || totalPoints == 0 {
			holders[i].SetProvisionalPayout(commonUtil.ZeroWei())
			continue
		}

		payout, err := payoutPool.Mul(holders[i].GetPoints()).Div(totalPoints)
		if err != nil {
			return nil, fmt.Errorf("could not determine holder payout: %w", err)
		}

		holders[i].SetProvisionalPayout(payout)
	}

	return holders, nil
}
