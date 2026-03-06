package gameserver

import (
	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.uber.org/mock/gomock"
)

type gameInputMatcher struct {
	expected types.GameInput
}

func (m gameInputMatcher) Matches(x interface{}) bool {
	actual, ok := x.(types.GameInput)
	if !ok {
		return false
	}

	if actual.GameID == uuid.Nil {
		return false
	}

	return actual.NumRounds == m.expected.NumRounds &&
		actual.DrawingDuration == m.expected.DrawingDuration &&
		actual.StakePerGame == m.expected.StakePerGame
}

func (m gameInputMatcher) String() string {
	return "matches game input"
}

func EqGameInput(expected types.GameInput) gomock.Matcher {
	return gameInputMatcher{expected: expected}
}
