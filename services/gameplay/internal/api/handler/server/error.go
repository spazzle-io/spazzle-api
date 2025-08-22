package server

import (
	"errors"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func handleServerDBError(dbError error) error {
	parsedDBError := db.ParseError(dbError)

	switch {
	// an unarchived server with the same name exists
	case parsedDBError.Code == db.UniqueViolationCode &&
		parsedDBError.ConstraintName == "servers_name_unique_unarchived_idx":
		return status.Error(codes.AlreadyExists, handler.ServerNameInUseError)

	// no server found
	case errors.Is(dbError, db.RecordNotFoundError):
		return status.Error(codes.NotFound, handler.ServerNotFoundError)

	// unknown/internal error
	default:
		return status.Error(codes.Internal, handler.InternalServerError)
	}
}
