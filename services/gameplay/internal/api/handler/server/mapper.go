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

	return pbServers, nil
}
