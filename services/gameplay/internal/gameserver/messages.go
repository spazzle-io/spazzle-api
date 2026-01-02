package gameserver

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
)

type WsMessage struct {
	ID         string              `json:"id,omitempty"`
	Type       string              `json:"type"`
	Timestamp  *time.Time          `json:"timestamp,omitempty"`
	StreamType eventbus.StreamType `json:"stream,omitempty"`
	Payload    json.RawMessage     `json:"payload"`
}

// OutgoingMessage represents a message that the GameServer sends to a client
// over the WebSocket connection. It supports optional delivery acknowledgment to the
// GameServer workflow.
type OutgoingMessage struct {
	WsMessage
	// CorrelationID uniquely identifies this message on the server side.
	// If RequiresWorkflowAck is false, CorrelationID may be empty.
	CorrelationID uuid.UUID
	// RequiresWorkflowAck indicates whether an acknowledgment message should be sent to the GameServer workflow
	// to notify whether the message was successfully delivered to the client. Note that an individual
	// acknowledgment message will be sent for each recipient of the message.
	//
	// If the message is successfully written to the client's WebSocket,
	// an acknowledgment is guaranteed to be sent.
	//
	// If the write fails or isn't able to be performed, a best-effort attempt may be made to send a
	// failure acknowledgment, but it is not guaranteed.
	RequiresWorkflowAck bool
}

// DirectMsgRecipient defines the recipient of a direct ws message.
type DirectMsgRecipient struct {
	// UserID is the target user.
	UserID uuid.UUID
	// ConnIDs, if non-empty, restricts delivery to the specified connection IDs.
	ConnIDs []uuid.UUID
	// ExcludeSpectators, if true, sends the message to only the user's non-spectating connection.
	// This filter applies even if ConnIDs is non-empty, so only listed connections
	// that are not spectating will receive the message.
	//
	// If false, and ConnIDs is empty, the message is sent to all
	// connections, including spectators.
	ExcludeSpectators bool
}

// DirectMsgPayload defines a direct ws message and its target recipients.
type DirectMsgPayload struct {
	Recipients []DirectMsgRecipient
	Msg        *OutgoingMessage
}
