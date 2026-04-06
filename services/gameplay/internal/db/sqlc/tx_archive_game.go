package db

import (
	"context"
	"fmt"
	"time"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

type GamePlayerResult struct {
	UserID            uuid.UUID
	Score             int32
	Pnl               commonUtil.Wei
	Position          int32
	RoundsPlayed      int32
	ProvisionalPayout commonUtil.Wei
	TotalStakeLost    commonUtil.Wei
	IsEvicted         bool
}

type ArchiveGameTxParams struct {
	GameID        uuid.UUID
	ServerID      uuid.UUID
	NumRounds     int32
	TotalPot      commonUtil.Wei
	GameStake     commonUtil.Wei
	PlayerResults []GamePlayerResult
	StartedAt     time.Time
	EndedAt       time.Time
}

type ArchiveGameTxResult struct {
	Game Game
}

func (store *SQLStore) ArchiveGameTx(ctx context.Context, params ArchiveGameTxParams) (ArchiveGameTxResult, error) {
	var result ArchiveGameTxResult

	err := store.execTx(ctx, func(queries *Queries) error {
		numPlayers, err := commonUtil.IntToInt32(len(params.PlayerResults))
		if err != nil {
			return fmt.Errorf("failed to cast num players in archive game tx: %w", err)
		}

		game, err := queries.CreateGame(ctx, CreateGameParams{
			ID:         params.GameID,
			ServerID:   params.ServerID,
			NumRounds:  params.NumRounds,
			NumPlayers: numPlayers,
			TotalPot: pgtype.Numeric{
				Int:   params.TotalPot.BigInt(),
				Valid: true,
			},
			GameStake: pgtype.Numeric{
				Int:   params.GameStake.BigInt(),
				Valid: true,
			},
			StartedAt: params.StartedAt,
			EndedAt:   params.EndedAt,
		})
		if err != nil {
			log.Error().Err(err).Msg("could not create game")

			dbError := ParseError(err)
			if dbError.Code == UniqueViolationCode {
				return ErrGameAlreadyExists
			}

			return fmt.Errorf("could not create game: %w", err)
		}

		result.Game = game

		gamePlayers := make([]InsertGamePlayersParams, 0, len(params.PlayerResults))
		for _, r := range params.PlayerResults {
			gamePlayers = append(gamePlayers, InsertGamePlayersParams{
				GameID: params.GameID,
				UserID: r.UserID,
				Score:  r.Score,
				Pnl: pgtype.Numeric{
					Int:   r.Pnl.BigInt(),
					Valid: true,
				},
				Position:     r.Position,
				RoundsPlayed: r.RoundsPlayed,
				ProvisionalPayout: pgtype.Numeric{
					Int:   r.ProvisionalPayout.BigInt(),
					Valid: true,
				},
				TotalStakeLost: pgtype.Numeric{
					Int:   r.TotalStakeLost.BigInt(),
					Valid: true,
				},
				IsEvicted: r.IsEvicted,
			})
		}

		_, err = queries.InsertGamePlayers(ctx, gamePlayers)
		if err != nil {
			log.Error().Err(err).Msg("could not insert game players")
			return fmt.Errorf("could not insert game players: %w", err)
		}

		for _, r := range params.PlayerResults {
			err = queries.UpsertUserStats(ctx, UpsertUserStatsParams{
				UserID: r.UserID,
				Score:  r.Score,
				Pnl: pgtype.Numeric{
					Int:   r.Pnl.BigInt(),
					Valid: true,
				},
				Volume: pgtype.Numeric{
					Int:   params.GameStake.BigInt(),
					Valid: true,
				},
			})
			if err != nil {
				log.Error().Err(err).Msg("could not upsert user stats")
				return fmt.Errorf("could not upsert user stats: %w", err)
			}

			err = queries.UpsertServerPlayerStats(ctx, UpsertServerPlayerStatsParams{
				ServerID: params.ServerID,
				UserID:   r.UserID,
				Score:    r.Score,
				Pnl: pgtype.Numeric{
					Int:   r.Pnl.BigInt(),
					Valid: true,
				},
				Volume: pgtype.Numeric{
					Int:   params.GameStake.BigInt(),
					Valid: true,
				},
			})
			if err != nil {
				log.Error().Err(err).Msg("could not upsert server player stats")
				return fmt.Errorf("could not upsert server player stats: %w", err)
			}
		}

		err = queries.UpdateServerGameStats(ctx, UpdateServerGameStatsParams{
			ServerID: params.ServerID,
			Volume: pgtype.Numeric{
				Int:   params.TotalPot.BigInt(),
				Valid: true,
			},
			NumPlayers: numPlayers,
		})
		if err != nil {
			log.Error().Err(err).Msg("could not update server game stats")
			return fmt.Errorf("could not update server game stats: %w", err)
		}

		return nil
	})

	return result, err
}
