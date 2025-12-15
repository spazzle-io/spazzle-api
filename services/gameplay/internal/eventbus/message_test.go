package eventbus

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEncodeMessage(t *testing.T) {
	msg := Message{
		Type:    "test_event",
		Payload: json.RawMessage(`{"key":"value"}`),
	}

	fields, err := encodeMessage(msg)
	require.NoError(t, err)

	require.Len(t, fields, 1)
	require.Contains(t, fields, redisFieldData)

	data, ok := fields[redisFieldData].([]byte)
	require.True(t, ok)

	var decoded messageData
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.Equal(t, "test_event", decoded.Type)
	require.Equal(t, json.RawMessage(`{"key":"value"}`), decoded.Payload)
	require.False(t, decoded.Timestamp.IsZero())
}

func TestEncodeMessageWithTargetFields(t *testing.T) {
	clientID := uuid.New()
	correlationID := uuid.New()

	msg := Message{
		Type:           "targeted_event",
		Payload:        json.RawMessage(`{}`),
		TargetClientID: &clientID,
		CorrelationID:  &correlationID,
	}
	fields, err := encodeMessage(msg)
	require.NoError(t, err)

	data := fields[redisFieldData].([]byte)
	var decoded messageData
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.Equal(t, &clientID, decoded.TargetClientID)
	require.Equal(t, &correlationID, decoded.CorrelationID)
}

func TestEncodeMessageWithNilPayload(t *testing.T) {
	msg := Message{
		Type:    "test_event",
		Payload: nil,
	}

	fields, err := encodeMessage(msg)
	require.NoError(t, err)

	data := fields[redisFieldData].([]byte)
	var decoded messageData
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.Equal(t, json.RawMessage("null"), decoded.Payload)
}

func TestDecodeMessageFromString(t *testing.T) {
	timestamp := time.Now().UTC().Truncate(time.Second)
	data := messageData{
		Type:      "test_event",
		Timestamp: timestamp,
		Payload:   json.RawMessage(`{"foo":"bar"}`),
	}
	jsonBytes, _ := json.Marshal(data)

	fields := map[string]interface{}{
		redisFieldData: string(jsonBytes),
	}

	msg, err := decodeMessage("1234-0", fields)
	require.NoError(t, err)

	require.Equal(t, "1234-0", msg.ID)
	require.Equal(t, "test_event", msg.Type)
	require.Equal(t, timestamp.Unix(), msg.Timestamp.Unix())
	require.Equal(t, json.RawMessage(`{"foo":"bar"}`), msg.Payload)
}

func TestDecodeMessageFromBytes(t *testing.T) {
	timestamp := time.Now().UTC().Truncate(time.Second)
	data := messageData{
		Type:      "test_event",
		Timestamp: timestamp,
		Payload:   json.RawMessage(`{"foo":"bar"}`),
	}
	jsonBytes, _ := json.Marshal(data)

	fields := map[string]interface{}{
		redisFieldData: jsonBytes,
	}

	msg, err := decodeMessage("1234-0", fields)
	require.NoError(t, err)

	require.Equal(t, "1234-0", msg.ID)
	require.Equal(t, "test_event", msg.Type)
}

func TestDecodeMessageWithTargetFields(t *testing.T) {
	clientID := uuid.New()
	correlationID := uuid.New()

	data := messageData{
		Type:           "targeted_event",
		Timestamp:      time.Now().UTC(),
		Payload:        json.RawMessage(`{}`),
		TargetClientID: &clientID,
		CorrelationID:  &correlationID,
	}
	jsonBytes, _ := json.Marshal(data)

	fields := map[string]interface{}{
		redisFieldData: string(jsonBytes),
	}

	msg, err := decodeMessage("1234-0", fields)
	require.NoError(t, err)

	require.Equal(t, &clientID, msg.TargetClientID)
	require.Equal(t, &correlationID, msg.CorrelationID)
}

func TestDecodeMessageMissingDataField(t *testing.T) {
	fields := map[string]interface{}{
		"wrong_field": "some_value",
	}

	_, err := decodeMessage("1234-0", fields)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid message 1234-0")
	require.Contains(t, err.Error(), "missing data field")
}

func TestDecodeMessageEmptyFields(t *testing.T) {
	fields := map[string]interface{}{}

	_, err := decodeMessage("1234-0", fields)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing data field")
}

func TestDecodeMessageInvalidDataType(t *testing.T) {
	fields := map[string]interface{}{
		redisFieldData: 12345,
	}

	_, err := decodeMessage("1234-0", fields)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid message 1234-0")
	require.Contains(t, err.Error(), "unexpected type int")
}

func TestDecodeMessageInvalidJSON(t *testing.T) {
	fields := map[string]interface{}{
		redisFieldData: "not valid json",
	}

	_, err := decodeMessage("1234-0", fields)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal message 1234-0")
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	clientID := uuid.New()
	correlationID := uuid.New()

	original := Message{
		Type:           "round_trip_test",
		Payload:        json.RawMessage(`{"nested":{"key":"value"},"array":[1,2,3]}`),
		TargetClientID: &clientID,
		CorrelationID:  &correlationID,
	}

	fields, err := encodeMessage(original)
	require.NoError(t, err)

	decoded, err := decodeMessage("5678-0", fields)
	require.NoError(t, err)

	require.Equal(t, "5678-0", decoded.ID)
	require.Equal(t, original.Type, decoded.Type)
	require.Equal(t, original.Payload, decoded.Payload)
	require.Equal(t, original.TargetClientID, decoded.TargetClientID)
	require.Equal(t, original.CorrelationID, decoded.CorrelationID)
	require.False(t, decoded.Timestamp.IsZero())
}

func TestEncodeDecodeWithNilOptionalFields(t *testing.T) {
	original := Message{
		Type:    "simple_event",
		Payload: json.RawMessage(`{}`),
	}

	fields, err := encodeMessage(original)
	require.NoError(t, err)

	decoded, err := decodeMessage("1-0", fields)
	require.NoError(t, err)

	require.Nil(t, decoded.TargetClientID)
	require.Nil(t, decoded.CorrelationID)
}
