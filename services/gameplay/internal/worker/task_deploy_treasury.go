package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/safekit/pkg/safe"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
)

const TaskDeployTreasury = "task:deploy_treasury"

type PayloadDeployTreasury struct {
	ServerID     uuid.UUID      `json:"server_id"`
	OwnerAddress common.Address `json:"owner_address"`
}

func (distributor *RedisTaskDistributor) DistributeTaskDeployTreasury(
	ctx context.Context,
	payload *PayloadDeployTreasury,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	task := asynq.NewTask(TaskDeployTreasury, jsonPayload, opts...)

	info, err := distributor.client.EnqueueContext(ctx, task, asynq.TaskID(payload.ServerID.String()))
	if err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			log.Info().
				Str("server_id", payload.ServerID.String()).
				Msg("deploy treasury task already enqueued, skipping")
			return nil
		}
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Bytes("payload", task.Payload()).
		Str("queue", info.Queue).
		Int("max_retry", info.MaxRetry).
		Msg("enqueued task")

	return nil
}

func (p *RedisTaskProcessor) processTaskDeployTreasury(ctx context.Context, task *asynq.Task) error {
	var payload PayloadDeployTreasury
	err := json.Unmarshal(task.Payload(), &payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal task payload: %v: %w", err, asynq.SkipRetry)
	}

	server, err := p.store.GetServerById(ctx, payload.ServerID)
	if err != nil {
		if errors.Is(err, db.RecordNotFoundError) {
			return fmt.Errorf("server not found: %v, %w", err, asynq.SkipRetry)
		}
		return fmt.Errorf("failed to get server: %w", err)
	}

	serverAddress, err := commonUtil.ParseWalletAddress(server.ServerAddress)
	if err != nil {
		return fmt.Errorf("failed to parse server address: %v, %w", err, asynq.SkipRetry)
	}

	treasury, err := p.store.GetTreasury(ctx, serverAddress.Hex())
	if err != nil {
		if errors.Is(err, db.RecordNotFoundError) {
			return fmt.Errorf("treasury not found: %v, %w", err, asynq.SkipRetry)
		}
		return fmt.Errorf("failed to get treasury: %w", err)
	}

	if treasury.Status == db.TreasuryStatusDeployed {
		return nil
	}

	predicted, err := p.treasuryClient.PredictAddress(payload.ServerID, payload.OwnerAddress)
	if err != nil {
		if errors.Is(err, safe.ErrVersionNotOnChain) {
			return fmt.Errorf("version not on chain: %w", asynq.SkipRetry)
		}
		return fmt.Errorf("failed to predict treasury address: %w", err)
	}

	if predicted != serverAddress {
		if _, err := p.store.MarkTreasuryFailed(ctx, serverAddress.Hex()); err != nil {
			log.Error().Err(err).Str("address", serverAddress.Hex()).Msg("failed to mark treasury as failed")
		}
		return fmt.Errorf("predicted %s does not match stored %s: %w",
			predicted.Hex(), serverAddress.Hex(), asynq.SkipRetry)
	}

	deployed, err := p.treasuryClient.IsDeployed(ctx, payload.ServerID, payload.OwnerAddress)
	if err != nil {
		log.Warn().Err(err).Str("address", serverAddress.Hex()).Msg("failed to determine if treasury was deployed")
	}
	if deployed {
		if _, err = p.store.RecoverDeployedTreasury(ctx, serverAddress.Hex()); err != nil {
			return fmt.Errorf("failed to recover deployed treasury: %w", err)
		}
		log.Info().Str("server_id", server.ID.String()).Msg("successfully recovered deployed treasury")
		return nil
	}

	if _, err = p.store.MarkTreasuryDeploying(ctx, serverAddress.Hex()); err != nil {
		return fmt.Errorf("failed to mark treasury as deploying: %w", err)
	}

	result, err := p.treasuryClient.Deploy(ctx, payload.ServerID, payload.OwnerAddress)
	if err != nil {
		if errors.Is(err, safe.ErrAddressAlreadyDeployed) {
			return fmt.Errorf("treasury already deployed on-chain, recovering on retry: %w", err)
		}

		if errors.Is(err, safe.ErrDeployTimeout) {
			return fmt.Errorf("treasury deployment timed out. recovering on retry: %w", err)
		}

		if errors.Is(err, safe.ErrTransactionReverted) {
			if _, markErr := p.store.MarkTreasuryFailed(ctx, serverAddress.Hex()); markErr != nil {
				log.Error().Err(markErr).Str("address", serverAddress.Hex()).Msg("failed to mark treasury as failed")
			}
			return fmt.Errorf("treasury deployment reverted on-chain: %w", asynq.SkipRetry)
		}

		var mismatchErr *safe.DeploymentMismatchError
		if errors.As(err, &mismatchErr) {
			if _, markErr := p.store.MarkTreasuryFailed(ctx, serverAddress.Hex()); markErr != nil {
				log.Error().Err(markErr).Str("address", serverAddress.Hex()).Msg("failed to mark treasury as failed")
			}
			log.Error().
				Str("predicted", mismatchErr.PredictedAddress.Hex()).
				Str("actual", mismatchErr.ActualAddress.Hex()).
				Str("tx_hash", mismatchErr.TxHash.Hex()).
				Msg("safekit deployment mismatch. This is a safekit bug!")
			return fmt.Errorf("deployment address mismatch: %w", asynq.SkipRetry)
		}

		retried, _ := asynq.GetRetryCount(ctx)
		maxRetry, _ := asynq.GetMaxRetry(ctx)
		if retried >= maxRetry {
			if _, markErr := p.store.MarkTreasuryFailed(ctx, serverAddress.Hex()); markErr != nil {
				log.Error().Err(markErr).Str("address", serverAddress.Hex()).Msg("failed to mark treasury as failed on exhaustion")
			}
			log.Error().
				Str("server_id", payload.ServerID.String()).
				Msg("treasury deployment failed after exhausting all retries")
		}

		return fmt.Errorf("failed to deploy treasury: %w", err)
	}

	blockNumber, err := commonUtil.Uint64ToInt64(result.BlockNumber)
	if err != nil {
		return fmt.Errorf("failed to convert block number to int64: %w", err)
	}

	gasUsed, err := commonUtil.Uint64ToInt64(result.GasUsed)
	if err != nil {
		return fmt.Errorf("failed to convert gas used to int64: %w", err)
	}

	_, err = p.store.MarkTreasuryDeployed(ctx, db.MarkTreasuryDeployedParams{
		Address: serverAddress.Hex(),
		TxHash: pgtype.Text{
			String: result.TxHash.Hex(),
			Valid:  true,
		},
		BlockNumber: pgtype.Int8{
			Int64: blockNumber,
			Valid: true,
		},
		GasUsed: pgtype.Int8{
			Int64: gasUsed,
			Valid: true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to mark treasury as deployed: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("server_id", payload.ServerID.String()).
		Str("address", result.Address.Hex()).
		Str("tx_hash", result.TxHash.Hex()).
		Uint64("block", result.BlockNumber).
		Uint64("gas_used", result.GasUsed).
		Msg("treasury deployed successfully")

	return nil
}
