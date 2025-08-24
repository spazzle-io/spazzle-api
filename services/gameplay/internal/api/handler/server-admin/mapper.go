package server_admin

import (
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapDBServerAdminToPb(serverAdmin *db.ServerAdmin) *pb.ServerAdmin {
	return &pb.ServerAdmin{
		ServerId: serverAdmin.ServerID.String(),
		UserId:   serverAdmin.UserID.String(),
		AddedAt:  timestamppb.New(serverAdmin.AddedAt),
	}
}

func mapDBServerAdminsToPb(serverAdmins []db.ServerAdmin) (pbServerAdmins []*pb.ServerAdmin) {
	for i := range serverAdmins {
		pbServerAdmin := mapDBServerAdminToPb(&serverAdmins[i])
		pbServerAdmins = append(pbServerAdmins, pbServerAdmin)
	}

	return
}
