package eventbus

import (
	"context"
	"encoding/json"
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

type Marker string

const (
	MarkerRoundEnded Marker = "round-ended"
)

var AllMarkers = []Marker{
	MarkerRoundEnded,
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

type EventBus interface {
	Session(game GameIdentifier) (Session, error)
	Replay(ctx context.Context, clientID uuid.UUID, game GameIdentifier, streamType StreamType, visibility ReplayVisibility, after string, limit int) (ReplayResult, error)
	MarkerID(ctx context.Context, game GameIdentifier, streamType StreamType, marker Marker) (string, error)
	Cleanup(ctx context.Context, game GameIdentifier) error
	Close() error
}

type Session interface {
	GameIdentifier() GameIdentifier

	Subscribe(ctx context.Context, streamType StreamType, startFrom StartPosition, handler MessageHandler) error
	Unsubscribe(streamType StreamType)

	Publish(ctx context.Context, streamType StreamType, msg PublishMessage) (messageID string, err error)

	Close()
}
