package db

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
)

type CreateServerTxParams struct {
	CreateServerParams
	ServerOwnerAddress common.Address
	AfterCreate        func(treasury ServerTreasury) error
}

type CreateServerTxResult struct {
	Server   Server
	Treasury ServerTreasury
}

func (store *SQLStore) CreateServerTx(ctx context.Context, params CreateServerTxParams) (CreateServerTxResult, error) {
	var result CreateServerTxResult

	err := store.execTx(ctx, func(queries *Queries) error {
		var err error

		result.Server, err = queries.CreateServer(ctx, params.CreateServerParams)
		if err != nil {
			return err
		}

		result.Treasury, err = queries.CreateTreasury(ctx, CreateTreasuryParams{
			Address:  params.ServerAddress,
			ServerID: result.Server.ID,
			Owner:    params.ServerOwnerAddress.Hex(),
		})
		if err != nil {
			return err
		}

		return params.AfterCreate(result.Treasury)
	})

	return result, err
}
