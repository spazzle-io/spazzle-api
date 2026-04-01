package game

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) GetGlobalLeaderboard(ctx context.Context, req *pb.GetGlobalLeaderboardRequest) (*pb.GetGlobalLeaderboardResponse, error) {
	page := req.GetPage()
	if page < 1 {
		page = 1
	}

	window := mapTimeWindowFromPb(req.GetTimeWindow())
	offset := leaderboardOffset(page)

	if !isWindowedLeaderboard(window) {
		return h.getGlobalLeaderboardAllTime(ctx, window, page, offset)
	}
	return h.getGlobalLeaderboardByWindow(ctx, window, page, offset)
}

func (h *Handler) getGlobalLeaderboardAllTime(ctx context.Context, window TimeWindow, page int32, offset int32) (*pb.GetGlobalLeaderboardResponse, error) {
	entries, err := h.store.GetGlobalLeaderboard(ctx, db.GetGlobalLeaderboardParams{
		PageOffset: offset,
		PageSize:   leaderboardPageSize,
	})
	if err != nil {
		log.Error().Err(err).Msg("could not fetch global leaderboard")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	totalCount, err := h.store.GetTotalUserStatsCount(ctx)
	if err != nil {
		log.Error().Err(err).Msg("could not fetch global leaderboard count")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	result, err := mapGlobalLeaderboardEntriesToPb(entries)
	if err != nil {
		log.Error().Err(err).Msg("could not map global leaderboard entries")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	return &pb.GetGlobalLeaderboardResponse{
		Players:    result,
		TotalCount: totalCount,
		TimeWindow: mapTimeWindowToPb(window),
		Page:       page,
	}, nil
}

func (h *Handler) getGlobalLeaderboardByWindow(ctx context.Context, window TimeWindow, page int32, offset int32) (*pb.GetGlobalLeaderboardResponse, error) {
	cacheWindow := mapTimeWindowToCacheWindow(window)

	var cached pb.GetGlobalLeaderboardResponse
	err := h.gameCache.GetLeaderboard(ctx, gamecache.LeaderboardScopeGlobal, uuid.Nil, cacheWindow, page, &cached)
	if err == nil {
		return &cached, nil
	}

	dbInterval := mapTimeWindowToDBInterval(window)
	entries, err := h.store.GetGlobalLeaderboardByWindow(ctx, db.GetGlobalLeaderboardByWindowParams{
		TimeWindow: dbInterval,
		PageOffset: offset,
		PageSize:   leaderboardPageSize,
	})
	if err != nil {
		log.Error().Err(err).Msg("could not fetch windowed global leaderboard")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	totalCount, err := h.store.GetGlobalLeaderboardByWindowCount(ctx, dbInterval)
	if err != nil {
		log.Error().Err(err).Msg("could not fetch windowed global leaderboard count")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	result, err := mapWindowedGlobalLeaderboardEntriesToPb(entries)
	if err != nil {
		log.Error().Err(err).Msg("could not map windowed global leaderboard entries")
		return nil, status.Error(codes.Internal, handler.InternalServerError)
	}

	response := &pb.GetGlobalLeaderboardResponse{
		Players:    result,
		TotalCount: totalCount,
		TimeWindow: mapTimeWindowToPb(window),
		Page:       page,
	}

	if err := h.gameCache.SetLeaderboard(ctx, gamecache.LeaderboardScopeGlobal, uuid.Nil, cacheWindow, page, response); err != nil {
		log.Warn().Err(err).Msg("could not cache windowed global leaderboard")
	}

	return response, nil
}
