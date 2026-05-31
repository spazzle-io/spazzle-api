package gameevents

import "github.com/google/uuid"

const (
	TypeConnectionInfo = "connection_info"

	TypeJoinGame = "join_game"

	TypePlayersJoined = "players_joined"
	TypePlayersLeft   = "players_left"
)

type ConnectionInfoPayload struct {
	ServerID     uuid.UUID `json:"server_id"`
	GameID       uuid.UUID `json:"game_id"`
	CurrentRound uint8     `json:"current_round"`
	UserID       uuid.UUID `json:"user_id"`
	ConnID       uuid.UUID `json:"conn_id"`
	Role         string    `json:"role"`
}

type JoinGamePayload struct {
	JoinCode string `json:"join_code"`
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
