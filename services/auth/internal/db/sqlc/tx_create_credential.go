package db

import "context"

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
			return err
		}

		return params.AfterCreate(result.Credential)
	})

	return result, err
}
