package workflow

import (
	"time"

	"go.temporal.io/sdk/log"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"

	"github.com/google/uuid"
)

type PlayerGameState struct {
	PlayerID     uuid.UUID
	Points       int64
	StakeLost    string
	RoundsPlayed uint8
	IsConnected  bool
	IsEjected    bool
	JoinedAt     time.Time
	LeftAt       time.Time
	EjectedAt    time.Time
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

type GameState struct {
	baseLogger log.Logger

	GameID       uuid.UUID
	StartedAt    time.Time
	EndedAt      time.Time
	IsTerminated bool

	Phase         types.Phase
	SubPhase      types.SubPhase
	NumRounds     int32
	CurrentRound  uint8
	GamePot       string
	StakePerGame  string
	StakePerRound string

	Players              map[uuid.UUID]*PlayerGameState
	MinNumPlayersToStart uint8
	DrawingDuration      time.Duration

	CurrentWord      types.Word
	CurrentArtist    uuid.UUID
	NextArtist       uuid.UUID
	DrawingStartedAt time.Time
	CorrectGuesses   map[uint8][]types.CorrectGuess
	CorrectGuessers  map[uint8]map[uuid.UUID]bool

	PastArtists map[uuid.UUID]bool
	PendingAcks map[uuid.UUID]*PendingAck

	GameServerInstances             map[uuid.UUID]*GameServerInstanceState
	GameServerInstancesLastPrunedAt time.Time

	PlayerReports  map[uuid.UUID]map[uuid.UUID]bool
	EjectedPlayers map[uuid.UUID]bool
}

func (gs *GameState) Logger() log.Logger {
	return log.With(gs.baseLogger, "GameID", gs.GameID, "Round", gs.CurrentRound)
}
