package workflow

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIsActivePlayer(t *testing.T) {
	testCases := []struct {
		name     string
		player   *PlayerGameState
		expected bool
	}{
		{
			name: "success",
			player: &PlayerGameState{
				IsConnected: true,
				IsEjected:   false,
			},
			expected: true,
		},
		{
			name: "disconnected player",
			player: &PlayerGameState{
				IsConnected: false,
				IsEjected:   false,
			},
			expected: false,
		},
		{
			name: "ejected player",
			player: &PlayerGameState{
				IsConnected: true,
				IsEjected:   true,
			},
			expected: false,
		},
		{
			name: "both disconnected and ejected",
			player: &PlayerGameState{
				IsConnected: false,
				IsEjected:   true,
			},
			expected: false,
		},
		{
			name:     "nil player",
			player:   nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isActivePlayer(tc.player)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestSortedUUIDs(t *testing.T) {
	id1 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	id2 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id3 := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	m := map[uuid.UUID]int{id1: 1, id2: 2, id3: 3}
	result := sortedUUIDs(m)

	require.Equal(t, []uuid.UUID{id2, id1, id3}, result)
}
