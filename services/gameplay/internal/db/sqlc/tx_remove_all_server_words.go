package db

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
)

type RemoveAllServerWordsTxResult struct {
	NumWordsRemoved int32
}

func (store *SQLStore) RemoveAllServerWordsTx(ctx context.Context, serverId uuid.UUID) (RemoveAllServerWordsTxResult, error) {
	var result RemoveAllServerWordsTxResult

	err := store.execTx(ctx, func(queries *Queries) error {
		var err error

		server, err := queries.GetServerById(ctx, serverId)
		if err != nil {
			log.Error().Err(err).Msg("could not get server")

			if errors.Is(err, RecordNotFoundError) {
				return ErrServerNotfound
			}

			return err
		}

		ct, err := queries.RemoveAllWordsFromServer(ctx, serverId)
		if err != nil {
			log.Error().Err(err).Msg("could not remove all words from server")
			return err
		}

		result.NumWordsRemoved, err = commonUtil.Int64ToInt32(ct.RowsAffected())
		if err != nil {
			return err
		}

		_, err = queries.UpdateServer(ctx, UpdateServerParams{
			ServerID: serverId,
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
