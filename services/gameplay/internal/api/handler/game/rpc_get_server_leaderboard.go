package game

import (
	"context"

	"buf.build/go/protovalidate"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) GetServerLeaderboard(ctx context.Context, req *pb.GetServerLeaderboardRequest) (*pb.GetServerLeaderboardResponse, error) {
	violations := validateGetServerLeaderboardRequest(req)
	if violations != nil {
		return nil, handler.InvalidArgumentError(violations)
	}

	serverID, err := uuid.Parse(req.GetServerId())
	if err != nil {
		log.Error().Err(err).Msg("invalid server id")
		return nil, status.Error(codes.InvalidArgument, handler.InvalidServerIdError)
	}

	_, err = h.Store.GetServerById(ctx, serverID)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch server")
		return nil, handler.HandleServerDBError(err)
	}

	page := req.GetPage()
	if page < 1 {
		page = 1
	}

	window := mapTimeWindowFromPb(req.GetTimeWindow())
	offset := leaderboardOffset(page)

	if !isWindowedLeaderboard(window) {
		return h.getServerLeaderboardAllTime(ctx, serverID, window, page, offset)
	}
	return h.getServerLeaderboardByWindow(ctx, serverID, window, page, offset)
}

func (h *Handler) getServerLeaderboardAllTime(ctx context.Context, serverID uuid.UUID, window TimeWindow, page int32, offset int32) (*pb.GetServerLeaderboardResponse, error) {
	entries, err := h.Store.GetServerLeaderboard(ctx, db.GetServerLeaderboardParams{
		ServerID:   serverID,
		PageOffset: offset,
		PageSize:   leaderboardPageSize,
	})
	if err != nil {
		log.Error().Err(err).Msg("could not fetch server leaderboard")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	totalCount, err := h.Store.GetTotalServerPlayerStatsCount(ctx, serverID)
	if err != nil {
		log.Error().Err(err).Msg("could not fetch server leaderboard count")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	result, err := mapServerLeaderboardEntriesToPb(entries)
	if err != nil {
		log.Error().Err(err).Msg("could not map server leaderboard entries")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	return &pb.GetServerLeaderboardResponse{
		Players:    result,
		TotalCount: totalCount,
		TimeWindow: mapTimeWindowToPb(window),
		Page:       page,
	}, nil
}

func (h *Handler) getServerLeaderboardByWindow(ctx context.Context, serverID uuid.UUID, window TimeWindow, page int32, offset int32) (*pb.GetServerLeaderboardResponse, error) {
	cacheWindow := mapTimeWindowToCacheWindow(window)

	var cached pb.GetServerLeaderboardResponse
	err := h.GameCache.GetLeaderboard(ctx, gamecache.LeaderboardScopeServer, serverID, cacheWindow, page, &cached)
	if err == nil {
		return &cached, nil
	}

	dbInterval := mapTimeWindowToDBInterval(window)
	entries, err := h.Store.GetServerLeaderboardByWindow(ctx, db.GetServerLeaderboardByWindowParams{
		ServerID:   serverID,
		TimeWindow: dbInterval,
		PageOffset: offset,
		PageSize:   leaderboardPageSize,
	})
	if err != nil {
		log.Error().Err(err).Msg("could not fetch windowed server leaderboard")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	totalCount, err := h.Store.GetServerLeaderboardByWindowCount(ctx, db.GetServerLeaderboardByWindowCountParams{
		ServerID:   serverID,
		TimeWindow: dbInterval,
	})
	if err != nil {
		log.Error().Err(err).Msg("could not fetch windowed server leaderboard count")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	result, err := mapWindowedServerLeaderboardEntriesToPb(entries)
	if err != nil {
		log.Error().Err(err).Msg("could not map windowed server leaderboard entries")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.GetServerLeaderboardResponse{
		Players:    result,
		TotalCount: totalCount,
		TimeWindow: mapTimeWindowToPb(window),
		Page:       page,
	}

	if err := h.GameCache.SetLeaderboard(ctx, gamecache.LeaderboardScopeServer, serverID, cacheWindow, page, response); err != nil {
		log.Warn().Err(err).Msg("could not cache windowed server leaderboard")
	}

	return response, nil
}

func validateGetServerLeaderboardRequest(req *pb.GetServerLeaderboardRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := protovalidate.Validate(req); err != nil {
		violations = append(violations, handler.ProtovalidateViolation(err)...)
	}

	return violations
}
