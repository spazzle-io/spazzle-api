package workflow

import (
	"strings"

	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
)

func getGameServerID(ctx workflow.Context) uuid.UUID {
	return uuid.MustParse(workflow.GetInfo(ctx).WorkflowExecution.ID)
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
