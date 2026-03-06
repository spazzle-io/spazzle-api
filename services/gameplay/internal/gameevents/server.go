package gameevents

import "github.com/google/uuid"

const (
	TypeConnectionInfo = "connection_info"

	TypeJoinGame = "join_game"

	TypePlayersJoined = "players_joined"
	TypePlayersLeft   = "players_left"

	TypeWarnPlayer           = "warn_player"
	TypePlayerWarned         = "player_warned"
	TypeReportPlayer         = "report_player"
	TypePlayersReported      = "players_reported"
	TypeClearPlayerReports   = "clear_player_reports"
	TypePlayerReportsCleared = "player_reports_cleared"
	TypeEjectPlayer          = "eject_player"
	TypePlayersEjected       = "players_ejected"
)

type ConnectionInfoPayload struct {
	ServerID     uuid.UUID `json:"server_id"`
	GameID       uuid.UUID `json:"game_id"`
	UserID       uuid.UUID `json:"user_id"`
	ConnID       uuid.UUID `json:"conn_id"`
	IsSpectating bool      `json:"is_spectating"`
}

type RejectionReason string

const (
	RejectionReasonInvalidGame   RejectionReason = "INVALID_GAME"
	RejectionReasonEjectedPlayer RejectionReason = "EJECTED_PLAYER"
)

type RejectedPlayer struct {
	PlayerID uuid.UUID       `json:"player_id"`
	Reason   RejectionReason `json:"reason"`
}

type PlayersJoinedPayload struct {
	AddedPlayers    []uuid.UUID      `json:"added_players,omitempty"`
	RejectedPlayers []RejectedPlayer `json:"rejected_players,omitempty"`
}

type PlayersLeftPayload struct {
	PlayerIDs []uuid.UUID `json:"player_ids"`
}

type WarnPlayerPayload struct {
	PlayerID uuid.UUID `json:"player_id"`
}

type PlayerWarnedPayload struct {
	PlayerID uuid.UUID `json:"player_id"`
}

type ReportPlayerPayload struct {
	ReportedID uuid.UUID `json:"reported_id"`
}

type PlayerReport struct {
	ReportedID   uuid.UUID `json:"reported_id"`
	ReporterID   uuid.UUID `json:"reporter_id"`
	IsArtist     bool      `json:"is_artist"`
	TotalReports int       `json:"total_reports"`
}

type PlayersReportedPayload struct {
	CurrentRound uint8          `json:"current_round"`
	Reports      []PlayerReport `json:"reports"`
}

type ClearPlayerReportsPayload struct {
	PlayerID uuid.UUID `json:"player_id"`
}

type PlayerReportsClearedPayload struct {
	PlayerIDs []uuid.UUID `json:"player_ids"`
}

type EjectPlayerPayload struct {
	PlayerID uuid.UUID `json:"player_id"`
}

type PlayerEjection struct {
	PlayerID     uuid.UUID `json:"player_id"`
	IsArtist     bool      `json:"is_artist"`
	EjectorID    uuid.UUID `json:"ejector_id"`
	TotalReports int       `json:"total_reports"`
}

type PlayersEjectedPayload struct {
	CurrentRound uint8            `json:"current_round"`
	Ejections    []PlayerEjection `json:"ejections"`
}
