package db

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const UniqueViolationCode = "23505"

var RecordNotFoundError = pgx.ErrNoRows

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
