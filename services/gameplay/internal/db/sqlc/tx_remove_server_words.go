package db

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
)

type RemoveServerWordsTxParams struct {
	ServerId uuid.UUID
	Words    []string
}

type RemoveServerWordsTxResult struct {
	NumWordsRemoved int32
}

func (store *SQLStore) RemoveServerWordsTx(ctx context.Context, params RemoveServerWordsTxParams) (RemoveServerWordsTxResult, error) {
	var result RemoveServerWordsTxResult

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

		ct, err := queries.RemoveWordsFromServer(ctx, RemoveWordsFromServerParams{
			ServerID: params.ServerId,
			Words:    params.Words,
		})
		if err != nil {
			log.Error().Err(err).Msg("could not remove words from server")
			return err
		}

		result.NumWordsRemoved, err = commonUtil.Int64ToInt32(ct.RowsAffected())
		if err != nil {
			return err
		}

		_, err = queries.UpdateServer(ctx, UpdateServerParams{
			ServerID: params.ServerId,
			NumCustomWords: pgtype.Int4{
				Int32: server.NumCustomWords - result.NumWordsRemoved,
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
