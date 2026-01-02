package workflow

import (
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"

	"github.com/google/uuid"
)

const (
	DefaultRoundNumber          = 1
	DefaultMinNumPlayersToStart = 1
)

type GameState struct {
	Phase       types.Phase
	SubPhase    types.SubPhase
	RoundNumber uint8
	StartedAt   time.Time
	SelectionID uuid.UUID

	Players              map[uuid.UUID]*PlayerState
	MinNumPlayersToStart uint8
	NumDrawingOptions    int32

	CurrentWord     types.Word
	CurrentArtist   uuid.UUID
	NextArtist      uuid.UUID
	CorrectGuesses  []CorrectGuess
	CorrectGuessers []uuid.UUID

	PastArtists map[uuid.UUID]bool
	PendingAcks map[uuid.UUID]*PendingAck

	GameServerInstances             map[uuid.UUID]*GameServerInstanceState
	GameServerInstancesLastPrunedAt time.Time
	GameServerInstanceTimeout       time.Duration

	PlayerReports map[uuid.UUID]map[uuid.UUID]bool
}

type CorrectGuess struct {
	PlayerID  uuid.UUID
	Timestamp time.Time
}

type PlayerState struct {
	PlayerID    uuid.UUID
	IsConnected bool
	JoinedAt    time.Time
}

type GameServerInstanceState struct {
	InstanceID uuid.UUID
	LastSeen   time.Time
}

type PendingAck struct {
	CorrelationID uuid.UUID
	ReceivedFrom  map[uuid.UUID]gameevents.AckStatus
	CreatedAt     time.Time
}
