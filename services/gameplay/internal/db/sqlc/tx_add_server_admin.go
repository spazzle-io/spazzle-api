package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rs/zerolog/log"
)

type AddServerAdminTxParams struct {
	ServerId uuid.UUID
	UserId   uuid.UUID
}

type AddServerAdminTxResult struct {
	ServerAdmin ServerAdmin
}

func (store *SQLStore) AddServerAdminTx(ctx context.Context, params AddServerAdminTxParams) (AddServerAdminTxResult, error) {
	var result AddServerAdminTxResult

	err := store.execTx(ctx, func(queries *Queries) error {
		var err error

		result.ServerAdmin, err = queries.AddServerAdmin(ctx, AddServerAdminParams{
			ServerID: params.ServerId,
			UserID:   params.UserId,
		})
		if err != nil {
			log.Error().Err(err).Msg("could not add server admin")

			dbError := ParseError(err)
			switch dbError.Code {
			case UniqueViolationCode:
				if dbError.ConstraintName == "server_admins_pkey" {
					return ErrUserAlreadyAdmin
				}
			case ForeignKeyViolationCode:
				if dbError.ConstraintName == "server_admins_server_id_fkey" {
					return ErrServerNotfound
				}
			}

			return err
		}

		server, err := queries.GetServerById(ctx, params.ServerId)
		if err != nil {
			return err
		}

		_, err = queries.UpdateServer(ctx, UpdateServerParams{
			ServerID: params.ServerId,
			NumAdmins: pgtype.Int4{
				Int32: server.NumAdmins + 1,
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
