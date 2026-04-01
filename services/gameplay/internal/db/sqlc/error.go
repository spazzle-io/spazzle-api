package db

import (
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	UniqueViolationCode     = "23505"
	ForeignKeyViolationCode = "23503"
)

var (
	RecordNotFoundError  = pgx.ErrNoRows
	ErrUserAlreadyAdmin  = errors.New("user is already a registered server admin")
	ErrServerNotfound    = errors.New("server not found")
	ErrGameAlreadyExists = errors.New("game already exists")
)

type Error struct {
	Code           string
	ConstraintName string
}

func ParseError(err error) *Error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		return &Error{
			Code:           pgErr.Code,
			ConstraintName: pgErr.ConstraintName,
		}
	}

	return &Error{}
}
