package game

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
)

const leaderboardPageSize = 20

type TimeWindow string

const (
	TimeWindowAllTime TimeWindow = "all_time"
	TimeWindowDaily   TimeWindow = "daily"
	TimeWindowWeekly  TimeWindow = "weekly"
	TimeWindowMonthly TimeWindow = "monthly"
)

func mapTimeWindowFromPb(window pb.TimeWindow) TimeWindow {
	switch window {
	case pb.TimeWindow_TIME_WINDOW_TODAY:
		return TimeWindowDaily
	case pb.TimeWindow_TIME_WINDOW_WEEKLY:
		return TimeWindowWeekly
	case pb.TimeWindow_TIME_WINDOW_MONTHLY:
		return TimeWindowMonthly
	case pb.TimeWindow_TIME_WINDOW_ALL_TIME:
		return TimeWindowAllTime
	default:
		return TimeWindowAllTime
	}
}

func mapTimeWindowToPb(w TimeWindow) pb.TimeWindow {
	switch w {
	case TimeWindowDaily:
		return pb.TimeWindow_TIME_WINDOW_TODAY
	case TimeWindowWeekly:
		return pb.TimeWindow_TIME_WINDOW_WEEKLY
	case TimeWindowMonthly:
		return pb.TimeWindow_TIME_WINDOW_MONTHLY
	case TimeWindowAllTime:
		return pb.TimeWindow_TIME_WINDOW_ALL_TIME
	default:
		return pb.TimeWindow_TIME_WINDOW_ALL_TIME
	}
}

func isWindowedLeaderboard(window TimeWindow) bool {
	return window != TimeWindowAllTime
}

func mapTimeWindowToCacheWindow(window TimeWindow) gamecache.LeaderboardWindow {
	switch window {
	case TimeWindowDaily:
		return gamecache.LeaderboardWindowDaily
	case TimeWindowWeekly:
		return gamecache.LeaderboardWindowWeekly
	case TimeWindowMonthly:
		return gamecache.LeaderboardWindowMonthly
	default:
		return gamecache.LeaderboardWindowDaily
	}
}

func mapTimeWindowToDBInterval(w TimeWindow) pgtype.Interval {
	switch w {
	case TimeWindowDaily:
		return pgtype.Interval{
			Days:  1,
			Valid: true,
		}
	case TimeWindowWeekly:
		return pgtype.Interval{
			Days:  7,
			Valid: true,
		}
	case TimeWindowMonthly:
		return pgtype.Interval{
			Months: 1,
			Valid:  true,
		}
	default:
		return pgtype.Interval{
			Days:  1,
			Valid: true,
		}
	}
}

func leaderboardOffset(page int32) int32 {
	if page < 1 {
		page = 1
	}

	return (page - 1) * leaderboardPageSize
}

func mapGlobalLeaderboardEntriesToPb(entries []db.UserStat) ([]*pb.LeaderboardEntry, error) {
	result := make([]*pb.LeaderboardEntry, 0, len(entries))

	for _, entry := range entries {
		pnl, err := db.ParseDBNumericWeiToStr(entry.TotalPnl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse total pnl: %w", err)
		}

		volume, err := db.ParseDBNumericWeiToStr(entry.TotalVolume)
		if err != nil {
			return nil, fmt.Errorf("failed to parse total volume: %w", err)
		}

		result = append(result, &pb.LeaderboardEntry{
			UserId:      entry.UserID.String(),
			TotalGames:  entry.TotalGames,
			TotalScore:  entry.TotalScore,
			TotalPnl:    pnl,
			TotalVolume: volume,
		})
	}

	return result, nil
}

func mapServerLeaderboardEntriesToPb(entries []db.ServerPlayerStat) ([]*pb.LeaderboardEntry, error) {
	result := make([]*pb.LeaderboardEntry, 0, len(entries))

	for _, entry := range entries {
		pnl, err := db.ParseDBNumericWeiToStr(entry.TotalPnl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse total pnl: %w", err)
		}

		volume, err := db.ParseDBNumericWeiToStr(entry.TotalVolume)
		if err != nil {
			return nil, fmt.Errorf("failed to parse total volume: %w", err)
		}

		result = append(result, &pb.LeaderboardEntry{
			UserId:      entry.UserID.String(),
			TotalGames:  entry.TotalGames,
			TotalScore:  entry.TotalScore,
			TotalPnl:    pnl,
			TotalVolume: volume,
		})
	}

	return result, nil
}

func mapWindowedGlobalLeaderboardEntriesToPb(entries []db.GetGlobalLeaderboardByWindowRow) ([]*pb.LeaderboardEntry, error) {
	result := make([]*pb.LeaderboardEntry, 0, len(entries))

	for _, entry := range entries {
		pnl, err := db.ParseDBNumericWeiToStr(entry.TotalPnl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse total pnl: %w", err)
		}

		volume, err := db.ParseDBNumericWeiToStr(entry.TotalVolume)
		if err != nil {
			return nil, fmt.Errorf("failed to parse total volume: %w", err)
		}

		result = append(result, &pb.LeaderboardEntry{
			UserId:      entry.UserID.String(),
			TotalGames:  entry.TotalGames,
			TotalScore:  entry.TotalScore,
			TotalPnl:    pnl,
			TotalVolume: volume,
		})
	}

	return result, nil
}

func mapWindowedServerLeaderboardEntriesToPb(entries []db.GetServerLeaderboardByWindowRow) ([]*pb.LeaderboardEntry, error) {
	result := make([]*pb.LeaderboardEntry, 0, len(entries))

	for _, entry := range entries {
		pnl, err := db.ParseDBNumericWeiToStr(entry.TotalPnl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse total pnl: %w", err)
		}

		volume, err := db.ParseDBNumericWeiToStr(entry.TotalVolume)
		if err != nil {
			return nil, fmt.Errorf("failed to parse total volume: %w", err)
		}

		result = append(result, &pb.LeaderboardEntry{
			UserId:      entry.UserID.String(),
			TotalGames:  entry.TotalGames,
			TotalScore:  entry.TotalScore,
			TotalPnl:    pnl,
			TotalVolume: volume,
		})
	}

	return result, nil
}

func mapGameLeaderboardEntriesToPb(entries []db.GamePlayer) ([]*pb.GamePlayerEntry, error) {
	result := make([]*pb.GamePlayerEntry, 0, len(entries))

	for _, entry := range entries {
		pnl, err := db.ParseDBNumericWeiToStr(entry.Pnl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pnl: %w", err)
		}

		payout, err := db.ParseDBNumericWeiToStr(entry.ProvisionalPayout)
		if err != nil {
			return nil, fmt.Errorf("failed to parse provisional payout: %w", err)
		}

		stakeLost, err := db.ParseDBNumericWeiToStr(entry.TotalStakeLost)
		if err != nil {
			return nil, fmt.Errorf("failed to parse total stake lost: %w", err)
		}

		result = append(result, &pb.GamePlayerEntry{
			UserId:            entry.UserID.String(),
			Score:             entry.Score,
			Pnl:               pnl,
			Position:          entry.Position,
			RoundsPlayed:      entry.RoundsPlayed,
			ProvisionalPayout: payout,
			TotalStakeLost:    stakeLost,
			IsEvicted:         entry.IsEvicted,
		})
	}

	return result, nil
}
