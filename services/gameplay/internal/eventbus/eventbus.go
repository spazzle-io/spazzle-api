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

type EventBus interface {
	Session(game GameIdentifier) (Session, error)
	Replay(ctx context.Context, game GameIdentifier, streamType StreamType, after string, limit int) (ReplayResult, error)
	Close() error
}

type Session interface {
	GameIdentifier() GameIdentifier

	Subscribe(ctx context.Context, streamType StreamType, startFrom StartPosition, handler MessageHandler) error
	Unsubscribe(streamType StreamType)

	Publish(ctx context.Context, streamType StreamType, msg PublishMessage) (messageID string, err error)

	Close()
}
