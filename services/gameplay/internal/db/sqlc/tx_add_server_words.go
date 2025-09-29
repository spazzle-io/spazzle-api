package db

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
)

type AddServerWordsTxParams struct {
	ServerId uuid.UUID
	Words    []string
}

type AddServerWordsTxResult struct {
	NumWordsAdded int32
}

func (store *SQLStore) AddServerWordsTx(ctx context.Context, params AddServerWordsTxParams) (AddServerWordsTxResult, error) {
	var result AddServerWordsTxResult

	err := store.execTx(ctx, func(queries *Queries) error {
		var err error

		server, err := queries.GetServerById(ctx, params.ServerId)
		if err != nil {
			log.Error().Err(err).Msg("could not get server")

			if errors.Is(err, RecordNotFoundError) {
				return ErrServerNotfound
			}

			return err
		}

		ct, err := queries.AddWordsToServer(ctx, AddWordsToServerParams{
			ServerID: params.ServerId,
			Words:    params.Words,
		})
		if err != nil {
			log.Error().Err(err).Msg("could not add words to server")
			return err
		}

		result.NumWordsAdded, err = commonUtil.Int64ToInt32(ct.RowsAffected())
		if err != nil {
			return err
		}

		_, err = queries.UpdateServer(ctx, UpdateServerParams{
			ServerID: params.ServerId,
			NumCustomWords: pgtype.Int4{
				Int32: server.NumCustomWords + result.NumWordsAdded,
				Valid: true,
			},
		})
		if err != nil {
			return err
		}

		return err
	})

	return result, err
}
