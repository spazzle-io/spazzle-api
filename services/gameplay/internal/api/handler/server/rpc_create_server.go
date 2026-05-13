package server

import (
	"context"
	"time"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/worker"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	deployTreasuryMaxRetries     = 10
	deployTreasuryTaskTimeout    = 5 * time.Minute
	deployTreasuryEnqueueTimeout = 3 * time.Second
)

// TODO: Add API validation for:
// Num rounds per game -
// Round duration secs -
// Num drawing options -

// TODO: Optimize redis streams to be pruned and archived after every round.

// TODO: How many concurrent redis streams can we have a time max? comfortable threshold?
// How to increase this threshold? Just increase redis instance size?

// TODO: Optimize temporal game workflow to make sure it is efficient.
// How many concurrent workflows can we have comfortably till we hit temporal limits?
// What is the maximum amount of players we can have in one temporal game workflow? At max concurrent workflows?

// TODO: Add API validation for max number of concurrent active games

// TODO: Add API validation for max number of concurrent players per game and maybe spectators/mods?

// TODO: What are our Infura limits? Worst case? Limits on the dev key?

// TODO: All notes go here.
// The point of all this is to get a baseline of the limits of the system, document them, and also document
// what strategies to be used when the limits are hit and also how to detect the limits are being approached.
//
// k8s and deployment
// - have consistent hashing
// - have a hpa on cpu, memory, and custom metric ws connections - scale up quick but scale down slow
// - have a cluster autoscaler
// - remember to tell nginx to buffer timeout for ws connections otherwise it will close them thinking they're idle http conns
// - remember to tell k8s never to kill all pods simultaneously even during deployment and maintenance
// -

func (h *Handler) CreateServer(ctx context.Context, req *pb.CreateServerRequest) (*pb.CreateServerResponse, error) {
	violations := validateCreateServerRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	tkPayload, err := h.AuthService.VerifyAccessToken(ctx, h.Config)
	if err != nil {
		log.Error().Err(err).Msg("access token verification failed")
		return nil, status.Error(codes.Unauthenticated, handler.UnauthorizedAccessError)
	}

	logger := log.With().Str("user_id", tkPayload.AccessTokenPayload.UserId).Logger()

	userId, err := uuid.Parse(tkPayload.AccessTokenPayload.UserId)
	if err != nil {
		logger.Error().Err(err).Msg("invalid user id")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	stakePerGame, err := commonUtil.NewNonNegativeWei(req.GetStakePerGame())
	if err != nil {
		logger.Error().Err(err).Msg("invalid stake per game")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidStakePerGameError)
	}

	serverID := uuid.New()

	ownerAddress, err := commonUtil.ParseWalletAddress(tkPayload.AccessTokenPayload.WalletAddress)
	if err != nil {
		logger.Error().
			Err(err).
			Str("owner_address", tkPayload.AccessTokenPayload.WalletAddress).
			Msg("invalid owner address")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	treasuryAddress, err := h.TreasuryClient.PredictAddress(serverID, ownerAddress)
	if err != nil {
		logger.Error().Err(err).Msg("failed to predict treasury address")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	params := db.CreateServerTxParams{
		CreateServerParams: db.CreateServerParams{
			ID:            serverID,
			Name:          req.GetName(),
			OwnerID:       userId,
			ServerAddress: treasuryAddress.Hex(),
			StakePerGame: pgtype.Numeric{
				Int:   stakePerGame.BigInt(),
				Valid: true,
			},
			NumRoundsPerGame:  req.GetNumRoundsPerGame(),
			RoundDurationSecs: req.GetRoundDurationSecs(),
			NumDrawingOptions: req.GetNumDrawingOptions(),
		},
		ServerOwnerAddress: ownerAddress,
		AfterCreate: func(treasury db.ServerTreasury) error {
			enqueueCtx, cancel := context.WithTimeout(ctx, deployTreasuryEnqueueTimeout)
			defer cancel()

			return h.TaskDistributor.DistributeTaskDeployTreasury(
				enqueueCtx,
				&worker.PayloadDeployTreasury{
					ServerID:     treasury.ServerID,
					OwnerAddress: ownerAddress,
				},
				asynq.MaxRetry(deployTreasuryMaxRetries),
				asynq.Timeout(deployTreasuryTaskTimeout),
			)
		},
	}
	result, err := h.Store.CreateServerTx(ctx, params)
	if err != nil {
		logger.Error().Err(err).Msg("create server tx failed")
		return nil, handler.HandleServerDBError(err)
	}

	pbServer, err := mapDBServerToPb(&result.Server)
	if err != nil {
		logger.Error().Err(err).Msg("failed to map db server to pb")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.CreateServerResponse{
		Server: pbServer,
	}

	logger.Info().Msg("server created successfully")

	return response, nil
}

func validateCreateServerRequest(req *pb.CreateServerRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}
