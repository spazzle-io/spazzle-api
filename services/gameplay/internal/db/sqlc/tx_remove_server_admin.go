package db

import (
	"context"
	"errors"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rs/zerolog/log"
)

type RemoveServerAdminTxParams struct {
	ServerId uuid.UUID
	UserId   uuid.UUID
}

func (store *SQLStore) RemoveServerAdminTx(ctx context.Context, params RemoveServerAdminTxParams) error {
	err := store.execTx(ctx, func(queries *Queries) error {
		var err error

		ct, err := queries.RemoveServerAdmin(ctx, RemoveServerAdminParams{
			ServerID: params.ServerId,
			UserID:   params.UserId,
		})
		if err != nil {
			log.Error().Err(err).Msg("could not add server admin")
			return err
		}

		numRemovedAdmins, err := commonUtil.Int64ToInt32(ct.RowsAffected())
		if err != nil {
			return err
		}

		server, err := queries.GetServerById(ctx, params.ServerId)
		if err != nil {
			log.Error().Err(err).Msg("could not get server")

			if errors.Is(err, RecordNotFoundError) {
				return ErrServerNotfound
			}

			return err
		}

		_, err = queries.UpdateServer(ctx, UpdateServerParams{
			ServerID: params.ServerId,
			NumAdmins: pgtype.Int4{
				Int32: server.NumAdmins - numRemovedAdmins,
				Valid: true,
			},
		})
		if err != nil {
			return err
		}

		return err
	})

	return err
}
