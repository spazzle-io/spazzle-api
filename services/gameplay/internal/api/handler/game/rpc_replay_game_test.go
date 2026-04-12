package game

import (
	"context"
	"errors"
	"testing"
	"time"

	authPb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/handler"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	mockeventbus "github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus/mock"
	mockservices "github.com/spazzle-io/spazzle-api/services/gameplay/internal/services/mock"
	pb "github.com/spazzle-io/spazzle-api/services/proto/gameplay/gameplay/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func generateReplayGameReqParams() *pb.ReplayGameRequest {
	return &pb.ReplayGameRequest{
		ServerId:   uuid.New().String(),
		GameId:     uuid.New().String(),
		StreamType: pb.StreamType_STREAM_TYPE_GAME_EVENTS,
		Limit:      800,
	}
}

func TestReplayGame(t *testing.T) {
	replayGameParams := generateReplayGameReqParams()
	require.NotEmpty(t, replayGameParams)

	testCases := []struct {
		name          string
		req           *pb.ReplayGameRequest
		buildStubs    func(bus *mockeventbus.MockEventBus, authService *mockservices.MockAuthGrpcService)
		checkResponse func(t *testing.T, res *pb.ReplayGameResponse, err error)
	}{
		{
			name: "success - authenticated user",
			req:  replayGameParams,
			buildStubs: func(bus *mockeventbus.MockEventBus, authService *mockservices.MockAuthGrpcService) {
				userID := uuid.New()

				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{
						AccessTokenPayload: &authPb.AccessTokenPayload{
							UserId: userID.String(),
						},
					}, nil)

				bus.EXPECT().
					MarkerID(gomock.Any(), gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Eq(eventbus.MarkerRoundEnded)).
					Times(1).
					Return("marker-id", nil)

				bus.EXPECT().
					Replay(gomock.Any(), gomock.Eq(userID), gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Eq(eventbus.ReplayVisibilityForClient), gomock.Eq("marker-id"), gomock.Eq(800)).
					Times(1).
					Return(eventbus.ReplayResult{
						Messages: []eventbus.Message{
							{
								ID:         "123",
								Type:       "some type",
								Timestamp:  time.Now().UTC(),
								StreamType: eventbus.GameEventsStreamType,
								Payload:    []byte(`{"word": "cat", "score": 100}`),
							},
						},
						HasMore: true,
						LastID:  "last-id",
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ReplayGameResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.NotEmpty(t, res.Messages)
				require.True(t, res.HasMore)
				require.NotEmpty(t, res.LastId)
			},
		},
		{
			name: "success - unauthenticated user",
			req:  replayGameParams,
			buildStubs: func(bus *mockeventbus.MockEventBus, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("unauthorized"))

				bus.EXPECT().
					MarkerID(gomock.Any(), gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Eq(eventbus.MarkerRoundEnded)).
					Times(1).
					Return("marker-id", nil)

				bus.EXPECT().
					Replay(gomock.Any(), gomock.Eq(uuid.Nil), gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Eq(eventbus.ReplayVisibilityBroadcastOnly), gomock.Eq("marker-id"), gomock.Eq(800)).
					Times(1).
					Return(eventbus.ReplayResult{
						Messages: []eventbus.Message{
							{
								ID:         "123",
								Type:       "some type",
								Timestamp:  time.Now().UTC(),
								StreamType: eventbus.GameEventsStreamType,
								Payload:    []byte(`{"word": "cat", "score": 100}`),
							},
						},
						HasMore: true,
						LastID:  "last-id",
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ReplayGameResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.NotEmpty(t, res.Messages)
				require.True(t, res.HasMore)
				require.NotEmpty(t, res.LastId)
			},
		},
		{
			name: "invalid request parameters",
			req: &pb.ReplayGameRequest{
				ServerId: "invalid",
				GameId:   "invalid",
				Limit:    1001,
			},
			buildStubs: func(bus *mockeventbus.MockEventBus, authService *mockservices.MockAuthGrpcService) {},
			checkResponse: func(t *testing.T, res *pb.ReplayGameResponse, err error) {
				require.Error(t, err)
				require.Empty(t, res)

				expectedFieldViolations := []string{"serverId", "gameId", "streamType", "limit"}
				handler.CheckInvalidRequestParams(t, err, expectedFieldViolations)
			},
		},
		{
			name: "user provided after parameter",
			req: &pb.ReplayGameRequest{
				ServerId:   uuid.New().String(),
				GameId:     uuid.New().String(),
				StreamType: pb.StreamType_STREAM_TYPE_GAME_EVENTS,
				After:      "1772996305636-0",
			},
			buildStubs: func(bus *mockeventbus.MockEventBus, authService *mockservices.MockAuthGrpcService) {
				authService.EXPECT().
					VerifyAccessToken(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&authPb.VerifyAccessTokenResponse{}, errors.New("unauthorized"))

				bus.EXPECT().
					Replay(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Eq(eventbus.GameEventsStreamType), gomock.Eq(eventbus.ReplayVisibilityBroadcastOnly), gomock.Eq("1772996305636-0"), gomock.Eq(defaultReplayLimit)).
					Times(1).
					Return(eventbus.ReplayResult{
						Messages: []eventbus.Message{
							{
								ID:         "123",
								Type:       "some type",
								Timestamp:  time.Now().UTC(),
								StreamType: eventbus.GameEventsStreamType,
								Payload:    []byte(`{"word": "cat", "score": 100}`),
							},
						},
						HasMore: true,
						LastID:  "last-id",
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.ReplayGameResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.NotEmpty(t, res.Messages)
				require.True(t, res.HasMore)
				require.NotEmpty(t, res.LastId)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deps := newTestDeps(t)

			tc.buildStubs(deps.bus, deps.authService)

			h := newTestHandler(deps)

			res, err := h.ReplayGame(context.Background(), tc.req)
			tc.checkResponse(t, res, err)
		})
	}
}
