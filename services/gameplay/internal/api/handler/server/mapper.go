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

	stakePerGameStr, err := db.ParseDBNumericWeiToStr(server.StakePerGame)
	if err != nil {
		return nil, err
	}

	return &pb.Server{
		Id:                server.ID.String(),
		Name:              server.Name,
		OwnerId:           server.OwnerID.String(),
		NumAdmins:         server.NumAdmins,
		NumCustomWords:    server.NumCustomWords,
		IsPubliclyVisible: server.IsPubliclyVisible,
		ServerAddress:     server.ServerAddress,
		StakePerGame:      stakePerGameStr,
		NumRoundsPerGame:  server.NumRoundsPerGame,
		RoundDurationSecs: server.RoundDurationSecs,
		NumDrawingOptions: server.NumDrawingOptions,
		IsArchived:        server.IsArchived,
		ArchivedAt:        archivedAt,
		CreatedAt:         timestamppb.New(server.CreatedAt),
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

	stakePerGameStr, err := db.ParseDBNumericWeiToStr(userServer.StakePerGame)
	if err != nil {
		return nil, err
	}

	return &pb.UserServer{
		Id:                userServer.ID.String(),
		Name:              userServer.Name,
		OwnerId:           userServer.OwnerID.String(),
		NumAdmins:         userServer.NumAdmins,
		NumCustomWords:    userServer.NumCustomWords,
		IsPubliclyVisible: userServer.IsPubliclyVisible,
		ServerAddress:     userServer.ServerAddress,
		StakePerGame:      stakePerGameStr,
		NumRoundsPerGame:  userServer.NumRoundsPerGame,
		RoundDurationSecs: userServer.RoundDurationSecs,
		NumDrawingOptions: userServer.NumDrawingOptions,
		IsAdmin:           userServer.IsAdmin,
		IsOwner:           userServer.IsOwner,
		IsArchived:        userServer.IsArchived,
		ArchivedAt:        archivedAt,
		CreatedAt:         timestamppb.New(userServer.CreatedAt),
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
