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
	PlayerIdx    uint32
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

	Phase                 types.Phase
	SubPhase              types.SubPhase
	NumRounds             uint8
	CurrentRound          uint8
	GamePot               string
	StakePerGame          string
	StakePerRound         string
	RoundStartedPublished map[uint8]struct{}

	Players              map[uuid.UUID]*PlayerGameState
	MinNumPlayersToStart uint8
	DrawingDuration      time.Duration

	CurrentWord      types.Word
	CurrentArtist    uuid.UUID
	NextArtist       uuid.UUID
	DrawingStartedAt time.Time
	CorrectGuesses   []types.CorrectGuess
	CorrectGuessers  map[uuid.UUID]struct{}

	PastArtists map[uuid.UUID]struct{}
	PendingAcks map[uuid.UUID]*PendingAck

	GameServerInstances             map[uuid.UUID]*GameServerInstanceState
	GameServerInstancesLastPrunedAt time.Time

	PlayerReportsMade  map[uuid.UUID][]uint32
	PlayerReportCounts map[uuid.UUID]uint32

	EjectedPlayers map[uuid.UUID]struct{}
}

func (gs *GameState) Logger() log.Logger {
	return log.With(gs.baseLogger, "GameID", gs.GameID, "Round", gs.CurrentRound)
}
