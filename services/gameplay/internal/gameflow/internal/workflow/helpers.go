package workflow

import (
	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
)

func getGameServerID(ctx workflow.Context) uuid.UUID {
	return uuid.MustParse(workflow.GetInfo(ctx).WorkflowExecution.ID)
}

func getGameID(ctx workflow.Context) uuid.UUID {
	return uuid.MustParse(workflow.GetInfo(ctx).WorkflowExecution.RunID)
}

func generateUUID(ctx workflow.Context) (uuid.UUID, error) {
	encodedCorrelationID := workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		return uuid.New()
	})

	var correlationID uuid.UUID
	err := encodedCorrelationID.Get(&correlationID)

	return correlationID, err
}
