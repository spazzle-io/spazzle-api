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
