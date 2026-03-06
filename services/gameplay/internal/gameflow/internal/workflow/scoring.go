package workflow

import (
	"math/big"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
)

const (
	TierArtist   = "artist"
	TierUnranked = "unranked"
)

type TierConfig struct {
	Name       string
	Threshold  float32
	BasePoints int16
}

var DefaultTierConfigs = []TierConfig{
	{Name: "top_20", Threshold: 0.20, BasePoints: 100},
	{Name: "top_50", Threshold: 0.50, BasePoints: 50},
	{Name: "top_80", Threshold: 0.80, BasePoints: 25},
}

type ScoringConfig struct {
	Tiers []TierConfig
	// Portion of top guesser's points to be awarded to the artist
	ArtistRewardRatio float64
	PayoutPercent     float32
}

var DefaultScoringConfig = ScoringConfig{
	Tiers:             DefaultTierConfigs,
	ArtistRewardRatio: 0.5,
	PayoutPercent:     0.9,
}

type PointsHolder interface {
	GetPoints() int64
	SetProvisionalPayout(payout string)
}

func CalculateProvisionalPayouts[T PointsHolder](state *GameState, holders []T) []T {
	var totalPoints int64
	for _, playerID := range sortedUUIDs(state.Players) {
		player := state.Players[playerID]
		totalPoints += player.Points
	}

	gamePotInt := commonUtil.ParseBigIntOrZero(state.GamePot)
	payoutPool := new(big.Int).Mul(gamePotInt, big.NewInt(int64(DefaultScoringConfig.PayoutPercent*100)))
	payoutPool = payoutPool.Div(payoutPool, big.NewInt(100))

	for i := range holders {
		if holders[i].GetPoints() == 0 || totalPoints == 0 {
			holders[i].SetProvisionalPayout("0")
			continue
		}

		payout := new(big.Int).Mul(payoutPool, big.NewInt(holders[i].GetPoints()))
		payout = payout.Div(payout, big.NewInt(totalPoints))
		holders[i].SetProvisionalPayout(commonUtil.BigIntString(payout))
	}

	return holders
}
