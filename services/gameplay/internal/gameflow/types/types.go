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
	NumDrawingOptions int32
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
	Length int
}

type Word struct {
	Text   string
	Tokens []Token
}

type GameStateView struct {
	Phase         Phase
	SubPhase      SubPhase
	RoundNumber   uint8
	CurrentArtist uuid.UUID
	CurrentWord   Word
}
