package gameevents

import (
	"github.com/google/uuid"
)

const (
	TypeGameEnded = "game_ended"
)

type PlayerFinalResult struct {
	PlayerID          uuid.UUID `json:"player_id"`
	Position          int       `json:"position"`
	TotalPoints       int64     `json:"total_points"`
	TotalStakeLost    string    `json:"total_stake_lost"`
	RoundsPlayed      uint8     `json:"rounds_played"`
	IsEjected         bool      `json:"is_evicted"`
	ProvisionalPayout string    `json:"provisional_payout"`
}

func (r *PlayerFinalResult) GetPoints() int64 {
	return r.TotalPoints
}

func (r *PlayerFinalResult) SetProvisionalPayout(payout string) {
	r.ProvisionalPayout = payout
}

type GameEndedPayload struct {
	TotalRounds uint8                `json:"total_rounds"`
	TotalPot    string               `json:"total_pot"`
	Results     []*PlayerFinalResult `json:"results"`
}
