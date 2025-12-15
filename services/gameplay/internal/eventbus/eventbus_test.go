package eventbus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartFromBeginning(t *testing.T) {
	pos := StartFromBeginning()
	require.Equal(t, "0", pos.String())
}

func TestStartFromNow(t *testing.T) {
	pos := StartFromNow()
	require.Equal(t, "$", pos.String())
}

func TestStartAfter(t *testing.T) {
	messageID := "1234567890-0"
	pos := StartAfter(messageID)
	require.Equal(t, messageID, pos.String())
}
