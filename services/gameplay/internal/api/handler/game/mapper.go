package game

import (
	"fmt"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
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

func mapCurrentGameToPb(currentGame *types.GameStateView) (pb.CurrentGameInfo, error) {
	numPlayers, err := commonUtil.IntToInt32(len(currentGame.Players))
	if err != nil {
		return pb.CurrentGameInfo{}, fmt.Errorf("failed to get num players in current game: %v", err)
	}

	return pb.CurrentGameInfo{
		Id:                currentGame.GameID.String(),
		Phase:             string(currentGame.Phase),
		SubPhase:          string(currentGame.SubPhase),
		CurrentRound:      uint32(currentGame.CurrentRound),
		NumRounds:         uint32(currentGame.NumRounds),
		CurrentArtist:     currentGame.CurrentArtist.String(),
		NumPlayers:        numPlayers,
		StartedAt:         timestamppb.New(currentGame.StartedAt),
		DrawingDurationMs: currentGame.DrawingDuration.Milliseconds(),
		StakePerGame:      currentGame.StakePerGame,
	}, nil
}

func mapRoundStandingToPb(playerResult *gameevents.PlayerRoundResult) *pb.RoundStanding {
	return &pb.RoundStanding{
		PlayerId:          playerResult.PlayerID.String(),
		GuessTimeMs:       playerResult.GuessTimeMs,
		Tier:              playerResult.Tier,
		RoundPosition:     int64(playerResult.RoundPosition),
		RoundPoints:       playerResult.RoundPoints,
		RoundStakeLost:    playerResult.RoundStakeLost,
		TotalPoints:       playerResult.TotalPoints,
		TotalStakeLost:    playerResult.TotalStakeLost,
		ProvisionalPayout: playerResult.ProvisionalPayout,
	}
}

func mapRoundStandingsToPb(playerResults []*gameevents.PlayerRoundResult) []*pb.RoundStanding {
	pbRoundStandings := make([]*pb.RoundStanding, 0, len(playerResults))

	for _, playerResult := range playerResults {
		pbRoundStandings = append(pbRoundStandings, mapRoundStandingToPb(playerResult))
	}

	return pbRoundStandings
}

func mapRoundSummaryToPb(payload gameevents.RoundEndedPayload) *pb.RoundSummary {
	return &pb.RoundSummary{
		Round:     uint32(payload.Round),
		ArtistId:  payload.ArtistID.String(),
		Word:      payload.Word,
		TotalPot:  payload.TotalPot,
		Standings: mapRoundStandingsToPb(payload.Results),
	}
}

func mapUserGamesToPb(userGames []db.ListUserGamesRow) ([]*pb.UserGameEntry, error) {
	pbUserGames := make([]*pb.UserGameEntry, 0, len(userGames))

	for _, game := range userGames {
		pnl, err := db.ParseDBNumericToWei(game.Pnl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pnl: %w", err)
		}

		totalPot, err := db.ParseDBNumericToWei(game.TotalPot)
		if err != nil {
			return nil, fmt.Errorf("failed to parse total pot: %w", err)
		}

		stakePerGame, err := db.ParseDBNumericToWei(game.GameStake)
		if err != nil {
			return nil, fmt.Errorf("failed to parse stake per game: %w", err)
		}

		provisionalPayout, err := db.ParseDBNumericToWei(game.ProvisionalPayout)
		if err != nil {
			return nil, fmt.Errorf("failed to parse provisional payout: %w", err)
		}

		stakeLost, err := db.ParseDBNumericToWei(game.TotalStakeLost)
		if err != nil {
			return nil, fmt.Errorf("failed to parse stake lost: %w", err)
		}

		pbUserGames = append(pbUserGames, &pb.UserGameEntry{
			Score:             game.Score,
			Pnl:               pnl.String(),
			ProvisionalPayout: provisionalPayout.String(),
			StakeLost:         stakeLost.String(),
			Position:          game.Position,
			RoundsPlayed:      game.RoundsPlayed,
			IsEvicted:         game.IsEvicted,
			Game: &pb.GameInfo{
				Id:           game.GameID.String(),
				ServerId:     game.ServerID.String(),
				NumRounds:    game.NumRounds,
				NumPlayers:   game.NumPlayers,
				TotalPot:     totalPot.String(),
				StakePerGame: stakePerGame.String(),
				StartedAt:    timestamppb.New(game.StartedAt),
				EndedAt:      timestamppb.New(game.EndedAt),
			},
		})
	}

	return pbUserGames, nil
}

func mapGameToPb(game db.Game) (*pb.GameInfo, error) {
	totalPot, err := db.ParseDBNumericToWei(game.TotalPot)
	if err != nil {
		return nil, fmt.Errorf("failed to parse total pot: %w", err)
	}

	stakePerGame, err := db.ParseDBNumericToWei(game.GameStake)
	if err != nil {
		return nil, fmt.Errorf("failed to parse stake per game: %w", err)
	}

	return &pb.GameInfo{
		Id:           game.ID.String(),
		ServerId:     game.ServerID.String(),
		NumRounds:    game.NumRounds,
		NumPlayers:   game.NumPlayers,
		TotalPot:     totalPot.String(),
		StakePerGame: stakePerGame.String(),
		StartedAt:    timestamppb.New(game.StartedAt),
		EndedAt:      timestamppb.New(game.EndedAt),
	}, nil
}

func mapGamesToPb(games []db.Game) ([]*pb.GameInfo, error) {
	pbGames := make([]*pb.GameInfo, 0, len(games))

	for _, game := range games {
		pbGame, err := mapGameToPb(game)
		if err != nil {
			return nil, err
		}

		pbGames = append(pbGames, pbGame)
	}

	return pbGames, nil
}
