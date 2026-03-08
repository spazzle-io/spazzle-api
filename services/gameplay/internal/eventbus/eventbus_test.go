package eventbus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllStreamTypes(t *testing.T) {
	if len(AllStreamTypes) != 2 {
		t.Fatalf("Did you add a new StreamType without updating AllStreamTypes?")
	}
}

func TestAllMarkers(t *testing.T) {
	if len(AllMarkers) != 1 {
		t.Fatalf("Did you add a new Marker without updating AllMarkers?")
	}
}

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
