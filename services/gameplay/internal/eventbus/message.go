package eventbus

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const redisFieldData = "data"

type messageData struct {
	Type           string          `json:"type"`
	Timestamp      time.Time       `json:"timestamp"`
	Payload        json.RawMessage `json:"payload"`
	TargetClientID *uuid.UUID      `json:"target_client_id,omitempty"`
	CorrelationID  *uuid.UUID      `json:"correlation_id,omitempty"`
}

func encodeMessage(msg Message) (map[string]interface{}, error) {
	data := messageData{
		Type:           msg.Type,
		Timestamp:      time.Now().UTC(),
		Payload:        msg.Payload,
		TargetClientID: msg.TargetClientID,
		CorrelationID:  msg.CorrelationID,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		redisFieldData: jsonBytes,
	}, nil
}

func decodeMessage(id string, fields map[string]interface{}) (Message, error) {
	dataField, ok := fields[redisFieldData]
	if !ok {
		return Message{}, fmt.Errorf("invalid message %s: missing %s field", id, redisFieldData)
	}

	var jsonBytes []byte
	switch val := dataField.(type) {
	case string:
		jsonBytes = []byte(val)
	case []byte:
		jsonBytes = val
	default:
		return Message{}, fmt.Errorf("invalid message %s: unexpected type %T", id, dataField)
	}

	var data messageData
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return Message{}, fmt.Errorf("failed to unmarshal message %s: %w", id, err)
	}

	return Message{
		ID:             id,
		Type:           data.Type,
		Timestamp:      data.Timestamp,
		Payload:        data.Payload,
		TargetClientID: data.TargetClientID,
		CorrelationID:  data.CorrelationID,
	}, nil
}
