package workflow

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSortedUUIDs(t *testing.T) {
	id1 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	id2 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id3 := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	m := map[uuid.UUID]int{id1: 1, id2: 2, id3: 3}
	result := sortedUUIDs(m)

	require.Equal(t, []uuid.UUID{id2, id1, id3}, result)
}
