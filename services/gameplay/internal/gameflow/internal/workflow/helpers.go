package workflow

import (
	"strings"

	"go.temporal.io/sdk/temporal"

	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
)

const (
	ErrTypeInvalidInput = "INVALID_INPUT"
	ErrTypeInvalidState = "INVALID_STATE"
)

func getGameServerID(ctx workflow.Context) uuid.UUID {
	return uuid.MustParse(workflow.GetInfo(ctx).WorkflowExecution.ID)
}

func isActivePlayer(player *PlayerGameState) bool {
	return player != nil && player.IsConnected && !player.IsEjected
}

func generateUUID(ctx workflow.Context) (uuid.UUID, error) {
	encodedCorrelationID := workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		return uuid.New()
	})

	var correlationID uuid.UUID
	err := encodedCorrelationID.Get(&correlationID)

	return correlationID, err
}

func sortedUUIDs[V any](m map[uuid.UUID]V) []uuid.UUID {
	return workflow.DeterministicKeysFunc(m, func(a, b uuid.UUID) int {
		return strings.Compare(a.String(), b.String())
	})
}

func nonRetryableErr(errType string, msg string, err error) error {
	return temporal.NewNonRetryableApplicationError(msg, errType, err)
}
