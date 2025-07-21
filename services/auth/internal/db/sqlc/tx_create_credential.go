package db

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"
)

var ErrCredentialAlreadyExists = errors.New("credential already exists")

type CreateCredentialTxParams struct {
	CreateCredentialParams
	AfterCreate func(credential Credential) error
}

type CreateCredentialTxResult struct {
	Credential Credential
}

func (store *SQLStore) CreateCredentialTx(
	ctx context.Context,
	params CreateCredentialTxParams,
) (CreateCredentialTxResult, error) {
	var result CreateCredentialTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.Credential, err = q.CreateCredential(ctx, params.CreateCredentialParams)
		if err != nil {
			log.Error().Err(err).Msg("could not create credential in db")
			dbError := ParseError(err)

			if dbError.Code == UniqueViolationCode {
				switch dbError.ConstraintName {
				case "credentials_user_id_key":
					return ErrCredentialAlreadyExists
				case "credentials_wallet_address_key":
					return ErrCredentialAlreadyExists
				}
			}

			return err
		}

		return params.AfterCreate(result.Credential)
	})

	return result, err
}
