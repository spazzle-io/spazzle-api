package types

import (
	"time"

	"github.com/google/uuid"
)

const (
	GameWorkflowTaskQueue = "game-workflow"

	GameServerInstanceTimeout   = time.Second * 15
	GameServerHeartbeatInterval = time.Second * 10

	WordSelectionTimeout = 15 * time.Second
)

type GameInput struct {
	GameID          uuid.UUID
	NumRounds       int32
	DrawingDuration time.Duration
	StakePerGame    string
}

type GameOutput struct{}

type Phase string

const (
	PhaseWaiting      Phase = "WAITING_FOR_PLAYERS"
	PhasePrepareRound Phase = "PREPARE_ROUND"
	PhaseInRound      Phase = "IN_ROUND"
	PhaseEndRound     Phase = "END_ROUND"
	PhaseEndGame      Phase = "END_GAME"
)

type SubPhase string

const (
	SubPhaseNone          SubPhase = ""
	SubPhaseConfirmArtist SubPhase = "CONFIRM_ARTIST"
	SubPhaseWordSelection SubPhase = "WORD_SELECTION"
	SubPhaseDrawing       SubPhase = "DRAWING"
)

type Token struct {
	Text   string
	Length int
}

type Word struct {
	Text   string
	Tokens []Token
}

type CorrectGuess struct {
	PlayerID  uuid.UUID
	Timestamp time.Time
}

type PlayerReport struct {
	ReporterID uuid.UUID
	ReportedID uuid.UUID
}

type PlayerEjection struct {
	PlayerID  uuid.UUID
	EjectorID uuid.UUID
}

type GameStateView struct {
	GameID            uuid.UUID
	StartedAt         time.Time
	EndedAt           time.Time
	Phase             Phase
	SubPhase          SubPhase
	CurrentRound      uint8
	CurrentArtist     uuid.UUID
	CurrentWord       Word
	Players           map[uuid.UUID]bool
	NumCorrectGuesses map[uint8]int
}
