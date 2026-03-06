package workflow

import (
	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
)

const (
	SignalPlayersJoin        = "players-join"
	SignalPlayersLeave       = "players-leave"
	SignalPlayersReported    = "players-reported"
	SignalPlayersEjected     = "players-ejected"
	SignalClearPlayerReports = "clear-player-reports"

	SignalGameServerInstanceHeartbeat    = "game-server-instance-heartbeat"
	SignalGameServerInstanceUnregistered = "game-server-instance-unregistered"

	SignalEventAck = "event-ack"

	SignalWordSelected   = "word-selected"
	SignalCorrectGuesses = "correct-guesses"

	SignalArtistDisconnected = "artist-disconnected"

	SignalTerminateGame = "terminate-game"
)

type PlayersJoinSignal struct {
	GameID    uuid.UUID
	PlayerIDs []uuid.UUID
}

type PlayersLeaveSignal struct {
	PlayerIDs []uuid.UUID
}

type PlayersEjectedSignal struct {
	Ejections []types.PlayerEjection
}

type ClearPlayerReportsSignal struct {
	PlayerIDs []uuid.UUID
}

type PlayersReportedSignal struct {
	Reports []types.PlayerReport
}

type GameServerInstanceHeartbeatSignal struct {
	InstanceID uuid.UUID
}

type GameServerInstanceUnregisteredSignal struct {
	InstanceID uuid.UUID
}

type WordSelectedSignal struct {
	GameID       uuid.UUID
	CurrentRound uint8
	Word         string
}

type CorrectGuessesSignal struct {
	GameID       uuid.UUID
	CurrentRound uint8
	Guesses      []types.CorrectGuess
}

type ArtistDisconnectedSignal struct {
	GameID       uuid.UUID
	CurrentRound uint8
	ArtistID     uuid.UUID
}

type TerminateGameSignal struct {
	GameID uuid.UUID
	Reason string
}
