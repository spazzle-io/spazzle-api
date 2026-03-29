package game

import (
	"fmt"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapGameRoleFromPb(gameRole pb.GameRole) (gameserver.Role, error) {
	var role gameserver.Role

	switch gameRole {
	case pb.GameRole_GAME_ROLE_PLAYER:
		role = gameserver.Player
	case pb.GameRole_GAME_ROLE_SPECTATOR:
		role = gameserver.Spectator
	case pb.GameRole_GAME_ROLE_MODERATOR:
		role = gameserver.Moderator
	default:
		return role, fmt.Errorf("unknown game role: %v", gameRole)
	}

	return role, nil
}

func mapStreamTypeToPb(streamType eventbus.StreamType) pb.StreamType {
	var st pb.StreamType

	switch streamType {
	case eventbus.GameEventsStreamType:
		st = pb.StreamType_STREAM_TYPE_GAME_EVENTS
	case eventbus.DrawingUpdatesStreamType:
		st = pb.StreamType_STREAM_TYPE_DRAWING_UPDATES
	default:
		st = pb.StreamType_STREAM_TYPE_UNSPECIFIED
	}

	return st
}

func mapStreamTypeFromPb(streamType pb.StreamType) (eventbus.StreamType, error) {
	var st eventbus.StreamType

	switch streamType {
	case pb.StreamType_STREAM_TYPE_GAME_EVENTS:
		st = eventbus.GameEventsStreamType
	case pb.StreamType_STREAM_TYPE_DRAWING_UPDATES:
		st = eventbus.DrawingUpdatesStreamType
	default:
		return st, fmt.Errorf("unknown stream type %v", streamType)
	}

	return st, nil
}

func mapEventBusMessageToPb(message eventbus.Message) (*pb.ReplayMessage, error) {
	payload := &structpb.Struct{}
	if len(message.Payload) > 0 && string(message.Payload) != "null" {
		if err := protojson.Unmarshal(message.Payload, payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %v", err)
		}
	}

	return &pb.ReplayMessage{
		Id:            message.ID,
		Type:          message.Type,
		Timestamp:     timestamppb.New(message.Timestamp),
		StreamType:    mapStreamTypeToPb(message.StreamType),
		Payload:       payload,
		CorrelationId: message.CorrelationID.String(),
	}, nil
}

func mapEventBusMessagesToPb(messages []eventbus.Message) ([]*pb.ReplayMessage, error) {
	pbReplayMessages := make([]*pb.ReplayMessage, 0, len(messages))
	for _, message := range messages {
		m, err := mapEventBusMessageToPb(message)
		if err != nil {
			return nil, err
		}

		pbReplayMessages = append(pbReplayMessages, m)
	}

	return pbReplayMessages, nil
}
