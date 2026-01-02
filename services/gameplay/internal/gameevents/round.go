package gameevents

import (
	"time"

	"github.com/google/uuid"
)

const (
	TypeArtistSelected  = "artist_selected"
	TypeArtistConfirmed = "artist_confirmed"

	TypeBeginWordSelection = "begin_word_selection"
	TypeGetWordChoices     = "get_word_choices"
	TypeSelectWord         = "select_word"
	TypeWordSelected       = "word_selected"
)

type ArtistSelectedPayload struct {
	ArtistID    uuid.UUID `json:"artist_id"`
	RoundNumber uint8     `json:"round_number"`
}

type ArtistConfirmedPayload struct {
	ArtistID    uuid.UUID `json:"artist_id"`
	RoundNumber uint8     `json:"round_number"`
}

type BeginWordSelectionPayload struct {
	ArtistID    uuid.UUID `json:"artist_id"`
	RoundNumber uint8     `json:"round_number"`
	EndsAt      time.Time `json:"ends_at"`
	SelectionID uuid.UUID `json:"selection_id"`
}

type GetWordChoicesPayload struct {
	Words []string `json:"words"`
}

type SelectWordPayload struct {
	Word string `json:"word"`
}

type WordToken struct {
	Length int
}

type WordSelectedPayload struct {
	RoundNumber uint8       `json:"round_number"`
	WordTokens  []WordToken `json:"word_tokens"`
}
