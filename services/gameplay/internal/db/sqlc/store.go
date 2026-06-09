package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	Querier
	AddServerAdminTx(ctx context.Context, params AddServerAdminTxParams) (AddServerAdminTxResult, error)
	RemoveServerAdminTx(ctx context.Context, params RemoveServerAdminTxParams) error
	AddServerWordsTx(ctx context.Context, params AddServerWordsTxParams) (AddServerWordsTxResult, error)
	RemoveServerWordsTx(ctx context.Context, params RemoveServerWordsTxParams) (RemoveServerWordsTxResult, error)
	RemoveAllServerWordsTx(ctx context.Context, serverId uuid.UUID) (RemoveAllServerWordsTxResult, error)
	ArchiveGameTx(ctx context.Context, params ArchiveGameTxParams) (ArchiveGameTxResult, error)
	CreateServerTx(ctx context.Context, params CreateServerTxParams) (CreateServerTxResult, error)
}

type SQLStore struct {
	*Queries
	connPool *pgxpool.Pool
}

func NewStore(connPool *pgxpool.Pool) Store {
	return &SQLStore{
		connPool: connPool,
		Queries:  New(connPool),
	}
}

func (store *SQLStore) execTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := store.connPool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			if !errors.Is(rbErr, pgx.ErrTxClosed) {
				log.Error().Err(rbErr).Msg("unexpected db tx rollback error")
			}
		}
	}()

	q := New(tx)
	if err = fn(q); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("transaction err: %v, rollback err: %v", err, rbErr)
		}

		return err
	}

	return tx.Commit(ctx)
}
