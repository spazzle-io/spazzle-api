package db

import "github.com/jackc/pgx/v5"

var RecordNotFoundError = pgx.ErrNoRows
