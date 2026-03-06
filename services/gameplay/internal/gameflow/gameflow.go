package gameflow

import (
	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
)

type Client interface {
	Game(gameServerID uuid.UUID, input types.GameInput) (gameID uuid.UUID, err error)
	GetGameState(gameServerID uuid.UUID) (state *types.GameStateView, err error)
	AcknowledgeGameEvent(gameServerID uuid.UUID, payload gameevents.EventAckPayload) error

	AddPlayers(gameServerID uuid.UUID, gameID uuid.UUID, playerIDs []uuid.UUID)
	RemovePlayers(gameServerID uuid.UUID, playerIDs []uuid.UUID)
	ReportPlayer(gameServerID uuid.UUID, reporter uuid.UUID, reported uuid.UUID)
	ClearPlayerReports(gameServerID uuid.UUID, playerID uuid.UUID)
	EjectPlayer(gameServerID uuid.UUID, playerID uuid.UUID, ejectorID uuid.UUID)

	SelectWord(gameServerID uuid.UUID, gameID uuid.UUID, currentRound uint8, word string) error
	RecordCorrectGuesses(gameServerID uuid.UUID, gameID uuid.UUID, currentRound uint8, correctGuesses []types.CorrectGuess)

	ArtistDisconnected(gameServerID uuid.UUID, gameID uuid.UUID, currentRound uint8, artistID uuid.UUID) error

	HeartbeatGameServerInstance(gameServerID uuid.UUID, gameID uuid.UUID, gameServerInstanceID uuid.UUID) error
	UnregisterGameServerInstance(gameServerID uuid.UUID, gameServerInstanceID uuid.UUID) error

	ShutdownGameServer(gameServerID uuid.UUID)
	Flush()
	Close()
}
