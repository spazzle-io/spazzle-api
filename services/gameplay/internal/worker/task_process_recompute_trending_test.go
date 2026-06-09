package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessTaskRecomputeTrending(t *testing.T) {
	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, err error)
	}{
		{
			name: "success",
			buildStubs: func(store *mockdb.MockStore) {
				trendingWindow := pgtype.Interval{
					Microseconds: trendingWindowMins * 60 * 1000 * 1000,
					Valid:        true,
				}

				store.EXPECT().
					RecomputeTrendingScores(gomock.Any(), gomock.Eq(trendingWindow)).
					Times(1).
					Return(nil)

				store.EXPECT().
					ResetTrendingScores(gomock.Any(), gomock.Eq(trendingWindow)).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "failed to recompute trending scores",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					RecomputeTrendingScores(gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("failed to recompute trending scores"))
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "failed to reset trending scores",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					RecomputeTrendingScores(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					ResetTrendingScores(gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("failed to reset trending scores"))
			},
			checkResponse: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)

			processor := &RedisTaskProcessor{
				store: store,
			}

			tc.buildStubs(store)

			err := processor.processTaskRecomputeTrending(context.Background(), nil)
			tc.checkResponse(t, err)
		})
	}
}
