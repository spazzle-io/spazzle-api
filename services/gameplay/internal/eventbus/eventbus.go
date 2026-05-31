package eventbus

import (
	"context"

	"github.com/google/uuid"
)

type EventBus interface {
	Session(game GameIdentifier) (Session, error)
	Replay(ctx context.Context, clientID uuid.UUID, game GameIdentifier, streamType StreamType, visibility ReplayVisibility, before string, after string, limit int) (ReplayResult, error)
	TrimStreamBefore(ctx context.Context, game GameIdentifier, streamType StreamType, upToID string) error
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
