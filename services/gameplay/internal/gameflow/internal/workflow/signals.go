package workflow

import "github.com/google/uuid"

const (
	SignalPlayersJoin                  = "players-join"
	SignalPlayersLeave                 = "players-leave"
	SignalGameServerInstanceHeartbeat  = "game-server-instance-heartbeat"
	SignalGameServerUnregisterInstance = "game-server-unregister-instance"
	SignalEventAck                     = "event-ack"
	SignalWordSelected                 = "word-selected"
	SignalCorrectGuesses               = "correct-guesses"
	SignalPlayerReport                 = "player-report"
	SignalArtistDisconnect             = "artist-disconnect"
)

type PlayersJoinSignal struct {
	PlayerIDs []uuid.UUID
}

type PlayersLeaveSignal struct {
	PlayerIDs []uuid.UUID
}

type GameServerInstanceHeartbeatSignal struct {
	InstanceID uuid.UUID
}

type GameServerInstanceUnregisterSignal struct {
	InstanceID uuid.UUID
}

type WordSelectedSignal struct {
	Word        string
	SelectionID uuid.UUID
}

type CorrectGuessesSignal struct {
	Guesses []CorrectGuess
}

type PlayerReportSignal struct {
	ReporterID     uuid.UUID
	ReportedID     uuid.UUID
	EscalateReport bool
}

type ArtistDisconnectSignal struct {
	ArtistID uuid.UUID
}
