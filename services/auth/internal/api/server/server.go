package server

import (
	"fmt"

	"github.com/spazzle-io/spazzle-api/services/auth/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/auth/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/token"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
)

type Server struct {
	handler.Handler
}

func New(config util.Config, store db.Store) (*Server, error) {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token maker: %w", err)
	}

	server := &Server{
		Handler: *handler.New(config, store, tokenMaker),
	}

	return server, nil
}
