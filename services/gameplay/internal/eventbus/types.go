package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type StreamType string

const (
	GameEventsStreamType     StreamType = "game-events"
	DrawingUpdatesStreamType StreamType = "drawing-updates"
)

var AllStreamTypes = []StreamType{
	GameEventsStreamType,
	DrawingUpdatesStreamType,
}

type MarkerType string

const (
	MarkerRoundStarted MarkerType = "round-started"
	MarkerRoundEnded   MarkerType = "round-ended"
)

type Marker struct {
	Type  MarkerType
	Round uint8
}

func (m Marker) String() string {
	return fmt.Sprintf("%s:%d", m.Type, m.Round)
}

type GameIdentifier struct {
	GameServerID uuid.UUID
	GameID       uuid.UUID
}

type StartPosition struct {
	position string
}

func StartFromBeginning() StartPosition {
	return StartPosition{position: "0"}
}

func StartFromNow() StartPosition {
	return StartPosition{position: "$"}
}

func StartAfter(messageID string) StartPosition {
	return StartPosition{position: messageID}
}

func (p StartPosition) String() string {
	return p.position
}

type PublishMessage struct {
	Timestamp      time.Time
	Type           string
	Payload        json.RawMessage
	TargetClientID uuid.UUID
	CorrelationID  uuid.UUID
	Marker         Marker
}

type Message struct {
	ID             string
	Timestamp      time.Time
	StreamType     StreamType
	Type           string
	Payload        json.RawMessage
	TargetClientID uuid.UUID
	CorrelationID  uuid.UUID
}

type MessageHandler func(ctx context.Context, msg Message)

type ReplayResult struct {
	Messages []Message
	HasMore  bool
	LastID   string
}

type ReplayVisibility int

const (
	ReplayVisibilityAll ReplayVisibility = iota
	ReplayVisibilityBroadcastOnly
	ReplayVisibilityForClient
)
