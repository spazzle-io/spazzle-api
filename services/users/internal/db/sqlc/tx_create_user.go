package db

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"
)

var (
	ErrUserAlreadyExists    = errors.New("user already exists")
	ErrGamerTagAlreadyInUse = errors.New("gamer tag already in use")
)

type CreateUserTxParams struct {
	CreateUserParams
	AfterCreate func() error
}

type CreateUserTxResult struct {
	User User
}

func (store *SQLStore) CreateUserTx(ctx context.Context, params CreateUserTxParams) (CreateUserTxResult, error) {
	var result CreateUserTxResult

	err := store.execTx(ctx, func(queries *Queries) error {
		var err error

		result.User, err = queries.CreateUser(ctx, params.CreateUserParams)
		if err != nil {
			log.Error().Err(err).Msg("could not create user in db")
			dbError := ParseError(err)

			if dbError.Code == UniqueViolationCode {
				switch dbError.ConstraintName {
				case "users_wallet_address_key":
					return ErrUserAlreadyExists
				case "users_gamer_tag_key":
					return ErrGamerTagAlreadyInUse
				}
			}

			return err
		}

		return params.AfterCreate()
	})

	return result, err
}
