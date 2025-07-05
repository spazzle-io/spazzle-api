package handler

import (
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth"
)

type Handler struct {
	pb.UnimplementedAuthServer

	config     util.Config
	store      db.Store
	tokenMaker token.Maker
}

func New(config util.Config, store db.Store, tokenMaker token.Maker) *Handler {
	return &Handler{
		config:     config,
		store:      store,
		tokenMaker: tokenMaker,
	}
}
