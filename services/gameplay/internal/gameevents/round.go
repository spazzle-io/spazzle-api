package gameevents

import (
	"time"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"

	"github.com/google/uuid"
)

const (
	TypeArtistSelected     = "artist_selected"
	TypeArtistConfirmed    = "artist_confirmed"
	TypeNextArtistSelected = "next_artist_selected"
	TypeArtistDisconnected = "artist_disconnected"
	TypeBeginWordSelection = "begin_word_selection"
	TypeGetWordChoices     = "get_word_choices"
	TypeSelectWord         = "select_word"
	TypeWordSelected       = "word_selected"
	TypeWordHintRevealed   = "word_hint_revealed"
	TypeGuessWord          = "guess_word"
	TypeWordGuessed        = "word_guessed"
	TypeBeginDrawing       = "begin_drawing"
	TypeUpdateDrawing      = "update_drawing"
	TypeEndDrawing         = "end_drawing"
	TypeRoundEnded         = "round_ended"
)

type ArtistSelectedPayload struct {
	ArtistID     uuid.UUID `json:"artist_id"`
	CurrentRound uint8     `json:"current_round"`
}

type ArtistConfirmedPayload struct {
	ArtistID     uuid.UUID `json:"artist_id"`
	CurrentRound uint8     `json:"current_round"`
}

type NextArtistSelectedPayload struct {
	PlayerID uuid.UUID `json:"player_id"`
	Round    uint8     `json:"round"`
}

type ArtistDisconnectedPayload struct {
	ArtistID     uuid.UUID `json:"artist_id"`
	CurrentRound uint8     `json:"current_round"`
}

type BeginWordSelectionPayload struct {
	ArtistID     uuid.UUID `json:"artist_id"`
	CurrentRound uint8     `json:"current_round"`
	EndsAt       time.Time `json:"ends_at"`
}

type GetWordChoicesPayload struct {
	Words []string `json:"words"`
}

type SelectWordPayload struct {
	Word string `json:"word"`
}

type WordToken struct {
	Length int `json:"length"`
}

type WordSelectedPayload struct {
	CurrentRound uint8       `json:"current_round"`
	WordTokens   []WordToken `json:"word_tokens"`
}

type WordHint struct {
	WordTokenIdx int    `json:"word_token_index"`
	CharIdx      int    `json:"character_index"`
	Char         string `json:"character"`
}

type WordHintRevealedPayload struct {
	CurrentRound uint8    `json:"current_round"`
	WordHint     WordHint `json:"word_hint"`
}

type GuessWordPayload struct {
	Guess string `json:"guess"`
}

type WordGuessedPayload struct {
	Guess        string    `json:"guess"`
	IsCorrect    bool      `json:"is_correct"`
	PlayerID     uuid.UUID `json:"player_id"`
	CurrentRound uint8     `json:"current_round"`
}

type BeginDrawingPayload struct {
	CurrentRound uint8     `json:"current_round"`
	EndsAt       time.Time `json:"ends_at"`
}

type UpdateDrawingPayload struct{} // TODO: Implement

type EndDrawingPayload struct {
	CurrentRound uint8  `json:"current_round"`
	Word         string `json:"word"`
}

type PlayerRoundResult struct {
	PlayerID          uuid.UUID `json:"player_id"`
	WasArtist         bool      `json:"was_artist"`
	GuessTimeMs       int64     `json:"guess_time_ms"`
	Tier              string    `json:"tier"`
	RoundPosition     int       `json:"round_position"`
	RoundPoints       int64     `json:"round_points"`
	RoundStakeLost    string    `json:"round_stake_lost"`
	TotalPoints       int64     `json:"total_points"`
	TotalStakeLost    string    `json:"total_stake_lost"`
	ProvisionalPayout string    `json:"provisional_payout"`
}

func (r *PlayerRoundResult) GetPoints() int64 {
	return r.TotalPoints
}

func (r *PlayerRoundResult) SetProvisionalPayout(payout commonUtil.Wei) {
	r.ProvisionalPayout = payout.String()
}

type RoundEndedPayload struct {
	Round           uint8                `json:"round"`
	ArtistID        uuid.UUID            `json:"artist_id"`
	Word            string               `json:"word"`
	DrawingDuration time.Duration        `json:"drawing_duration"`
	Results         []*PlayerRoundResult `json:"results"`
	TotalPot        string               `json:"total_pot"`
	IsFinalRound    bool                 `json:"is_final_round"`
}
