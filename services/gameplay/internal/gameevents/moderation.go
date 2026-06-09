package gameevents

import "github.com/google/uuid"

const (
	TypeWarnPlayer   = "warn_player"
	TypePlayerWarned = "player_warned"

	TypeReportPlayer    = "report_player"
	TypePlayersReported = "players_reported"

	TypeClearPlayerReports   = "clear_player_reports"
	TypePlayerReportsCleared = "player_reports_cleared"

	TypeEjectPlayer    = "eject_player"
	TypePlayersEjected = "players_ejected"
)

type WarnPlayerPayload struct {
	PlayerID uuid.UUID `json:"player_id"`
}

type PlayerWarnedPayload struct {
	PlayerID uuid.UUID `json:"player_id"`
}

type ReportPlayerPayload struct {
	ReportTarget uuid.UUID `json:"report_target"`
}

type PlayerReport struct {
	ReportedPlayer uuid.UUID `json:"reported_player"`
	Reporter       uuid.UUID `json:"reporter"`
	IsArtist       bool      `json:"is_artist"`
	TotalReports   uint32    `json:"total_reports"`
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
	Ejector      uuid.UUID `json:"ejector"`
	TotalReports uint32    `json:"total_reports"`
}

type PlayersEjectedPayload struct {
	CurrentRound uint8            `json:"current_round"`
	Ejections    []PlayerEjection `json:"ejections"`
}
