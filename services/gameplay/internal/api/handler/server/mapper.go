package server

import (
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapDBServerToPb(server *db.Server) (*pb.Server, error) {
	var archivedAt *timestamppb.Timestamp
	if server.ArchivedAt.Valid {
		archivedAt = timestamppb.New(server.ArchivedAt.Time)
	}

	stakePerGame, err := db.ParseDBNumericToWei(server.StakePerGame)
	if err != nil {
		return nil, err
	}

	totalVolume, err := db.ParseDBNumericToWei(server.TotalVolume)
	if err != nil {
		return nil, err
	}

	return &pb.Server{
		Id:                server.ID.String(),
		Name:              server.Name,
		OwnerId:           server.OwnerID.String(),
		NumAdmins:         server.NumAdmins,
		NumCustomWords:    server.NumCustomWords,
		ServerAddress:     server.ServerAddress,
		StakePerGame:      stakePerGame.String(),
		NumRoundsPerGame:  server.NumRoundsPerGame,
		RoundDurationSecs: server.RoundDurationSecs,
		NumDrawingOptions: server.NumDrawingOptions,
		IsArchived:        server.IsArchived,
		ArchivedAt:        archivedAt,
		CreatedAt:         timestamppb.New(server.CreatedAt),
		TotalGames:        server.TotalGames,
		TotalVolume:       totalVolume.String(),
		TotalPlayers:      server.TotalPlayers,
		TrendingScore:     server.TrendingScore,
	}, nil
}

func mapDBServersToPb(servers []db.Server) (pbServers []*pb.Server, err error) {
	for i := range servers {
		pbServer, err := mapDBServerToPb(&servers[i])
		if err != nil {
			return nil, err
		}

		pbServers = append(pbServers, pbServer)
	}

	return
}

func mapDBUserServerToPb(userServer *db.ListUserServersRow) (*pb.UserServer, error) {
	var archivedAt *timestamppb.Timestamp
	if userServer.ArchivedAt.Valid {
		archivedAt = timestamppb.New(userServer.ArchivedAt.Time)
	}

	stakePerGame, err := db.ParseDBNumericToWei(userServer.StakePerGame)
	if err != nil {
		return nil, err
	}

	totalVolume, err := db.ParseDBNumericToWei(userServer.TotalVolume)
	if err != nil {
		return nil, err
	}

	return &pb.UserServer{
		Id:                userServer.ID.String(),
		Name:              userServer.Name,
		OwnerId:           userServer.OwnerID.String(),
		NumAdmins:         userServer.NumAdmins,
		NumCustomWords:    userServer.NumCustomWords,
		ServerAddress:     userServer.ServerAddress,
		StakePerGame:      stakePerGame.String(),
		NumRoundsPerGame:  userServer.NumRoundsPerGame,
		RoundDurationSecs: userServer.RoundDurationSecs,
		NumDrawingOptions: userServer.NumDrawingOptions,
		IsAdmin:           userServer.IsAdmin,
		IsOwner:           userServer.IsOwner,
		IsArchived:        userServer.IsArchived,
		ArchivedAt:        archivedAt,
		CreatedAt:         timestamppb.New(userServer.CreatedAt),
		TotalGames:        userServer.TotalGames,
		TotalVolume:       totalVolume.String(),
		TotalPlayers:      userServer.TotalPlayers,
		TrendingScore:     userServer.TrendingScore,
	}, nil
}

func mapDBUserServersToPb(userServers []db.ListUserServersRow) (pbUserServers []*pb.UserServer, err error) {
	for i := range userServers {
		pbUserServer, err := mapDBUserServerToPb(&userServers[i])
		if err != nil {
			return nil, err
		}

		pbUserServers = append(pbUserServers, pbUserServer)
	}

	return
}

func mapDBServerTreasuryStatusToPb(status db.TreasuryStatus) pb.ServerTreasuryStatus {
	switch status {
	case db.TreasuryStatusPending:
		return pb.ServerTreasuryStatus_SERVER_TREASURY_STATUS_PENDING
	case db.TreasuryStatusDeploying:
		return pb.ServerTreasuryStatus_SERVER_TREASURY_STATUS_DEPLOYING
	case db.TreasuryStatusDeployed:
		return pb.ServerTreasuryStatus_SERVER_TREASURY_STATUS_DEPLOYED
	case db.TreasuryStatusFailed:
		return pb.ServerTreasuryStatus_SERVER_TREASURY_STATUS_FAILED
	default:
		return pb.ServerTreasuryStatus_SERVER_TREASURY_STATUS_UNSPECIFIED
	}
}

func mapDBServerTreasuryToPb(treasury db.ServerTreasury) *pb.ServerTreasury {
	return &pb.ServerTreasury{
		Address:      treasury.Address,
		ServerId:     treasury.ServerID.String(),
		Status:       mapDBServerTreasuryStatusToPb(treasury.Status),
		OwnerAddress: treasury.Owner,
		TxHash:       treasury.TxHash.String,
		BlockNumber:  treasury.BlockNumber.Int64,
		GasUsed:      treasury.GasUsed.Int64,
		DeployedAt:   timestamppb.New(treasury.DeployedAt.Time),
		CreatedAt:    timestamppb.New(treasury.CreatedAt),
		UpdatedAt:    timestamppb.New(treasury.UpdatedAt),
	}
}
