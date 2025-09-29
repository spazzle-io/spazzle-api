package word

import (
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapWordStoreWordToPb(word wordstore.Word) *pb.Word {
	return &pb.Word{
		Id:       word.Id.String(),
		ServerId: word.ServerID.String(),
		Word:     word.Word,
		AddedAt:  timestamppb.New(word.AddedAt),
	}
}

func mapWordStoreWordsToPb(words []wordstore.Word) (pbWords []*pb.Word) {
	for i := range words {
		pbWord := mapWordStoreWordToPb(words[i])
		pbWords = append(pbWords, pbWord)
	}

	return
}

func mapDBWordToPb(word *db.ListWordsRow) *pb.Word {
	return &pb.Word{
		Id:       word.ID.String(),
		ServerId: word.ServerID.String(),
		Word:     word.Word,
		AddedAt:  timestamppb.New(word.AddedAt),
	}
}

func mapDBWordsToPb(words []db.ListWordsRow) (pbWords []*pb.Word) {
	for i := range words {
		pbWord := mapDBWordToPb(&words[i])
		pbWords = append(pbWords, pbWord)
	}

	return
}
