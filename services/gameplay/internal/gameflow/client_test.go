package gameflow

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/workflow"
	mockgameflow "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/mock"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/enums/v1"
	temporalclient "go.temporal.io/sdk/client"
	"go.uber.org/mock/gomock"
)

type workflowRun struct {
	workflowRunID string
}

func newWorkflowRun(workflowRunID string) *workflowRun {
	return &workflowRun{workflowRunID: workflowRunID}
}

func (r *workflowRun) GetID() string {
	return ""
}

func (r *workflowRun) GetRunID() string {
	return r.workflowRunID
}

func (r *workflowRun) Get(_ context.Context, _ interface{}) error {
	return nil
}

func (r *workflowRun) GetWithOptions(_ context.Context, _ interface{}, _ temporalclient.WorkflowRunGetOptions) error {
	return nil
}

type encodedValue struct {
	valuePtr interface{}
	err      error
}

func newEncodedValue(v interface{}, err error) *encodedValue {
	return &encodedValue{v, err}
}

func (ev *encodedValue) HasValue() bool {
	return ev.valuePtr != nil
}

func (ev *encodedValue) Get(valuePtr interface{}) error {
	if ev.err != nil {
		return ev.err
	}

	dest := reflect.ValueOf(valuePtr).Elem()
	src := reflect.ValueOf(ev.valuePtr)
	if src.Kind() == reflect.Ptr {
		src = src.Elem()
	}

	dest.Set(src)
	return nil
}

func withMockTemporalDial(t *testing.T, fn func(opts temporalclient.Options) (temporalclient.Client, error)) {
	t.Helper()

	initialTemporalDial := temporalDial
	temporalDial = fn

	t.Cleanup(func() {
		temporalDial = initialTemporalDial
	})
}

func createTestClient(t *testing.T) (*mockgameflow.MockTemporal, Client) {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	temporal := mockgameflow.NewMockTemporal(ctrl)
	withMockTemporalDial(t, func(opts temporalclient.Options) (temporalclient.Client, error) {
		return temporal, nil
	})

	config := util.Config{
		ServiceName: "test",
		Environment: util.Development,
	}
	client, err := NewClient(config, WithoutAutoFlush())
	require.NoError(t, err)
	require.NotNil(t, client)

	return temporal, client
}

func TestGame(t *testing.T) {
	gameID := uuid.New()
	gameServerID := uuid.New()
	workflowRunID := "some-workflow-run-id"

	gameWorkflowInput := types.GameInput{
		GameID:          gameID,
		NumRounds:       10,
		DrawingDuration: time.Minute,
		StakePerGame:    "1000000000000000000",
	}

	workflowOpts := temporalclient.StartWorkflowOptions{
		ID:                       gameServerID.String(),
		TaskQueue:                types.GameWorkflowTaskQueue,
		WorkflowIDConflictPolicy: enums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
	}

	testCases := []struct {
		name          string
		buildStubs    func(temporal *mockgameflow.MockTemporal)
		checkResponse func(t *testing.T, outputGameID uuid.UUID, err error)
	}{
		{
			name: "success",
			buildStubs: func(temporal *mockgameflow.MockTemporal) {
				workflowRun := newWorkflowRun(workflowRunID)

				temporal.EXPECT().
					ExecuteWorkflow(gomock.Any(), gomock.Eq(workflowOpts), gomock.Any(), gomock.Eq(gameWorkflowInput)).
					Times(1).
					Return(workflowRun, nil)

				gameStateView := types.GameStateView{
					GameID: gameID,
					Phase:  types.PhaseInRound,
				}

				encodedValue := newEncodedValue(&gameStateView, nil)

				temporal.EXPECT().
					QueryWorkflow(gomock.Any(), gomock.Eq(gameServerID.String()), gomock.Eq(""), gomock.Eq(workflow.QueryGetGameState)).
					Times(1).
					Return(encodedValue, nil)
			},
			checkResponse: func(t *testing.T, outputGameID uuid.UUID, err error) {
				require.NoError(t, err)
				require.Equal(t, gameID, outputGameID)
			},
		},
		{
			name: "error calling ExecuteWorkflow",
			buildStubs: func(temporal *mockgameflow.MockTemporal) {
				temporal.EXPECT().
					ExecuteWorkflow(gomock.Any(), gomock.Eq(workflowOpts), gomock.Any(), gomock.Eq(gameWorkflowInput)).
					Times(1).
					Return(nil, errors.New("error calling ExecuteWorkflow"))
			},
			checkResponse: func(t *testing.T, outputGameID uuid.UUID, err error) {
				require.Error(t, err)
				require.Empty(t, outputGameID)
			},
		},
		{
			name: "error querying game state",
			buildStubs: func(temporal *mockgameflow.MockTemporal) {
				workflowRun := newWorkflowRun(workflowRunID)

				temporal.EXPECT().
					ExecuteWorkflow(gomock.Any(), gomock.Eq(workflowOpts), gomock.Any(), gomock.Eq(gameWorkflowInput)).
					Times(1).
					Return(workflowRun, nil)

				temporal.EXPECT().
					QueryWorkflow(gomock.Any(), gomock.Eq(gameServerID.String()), gomock.Eq(""), gomock.Eq(workflow.QueryGetGameState)).
					Times(1).
					Return(nil, errors.New("error querying game state"))
			},
			checkResponse: func(t *testing.T, outputGameID uuid.UUID, err error) {
				require.Error(t, err)
				require.Empty(t, outputGameID)
			},
		},
		{
			name: "game ending",
			buildStubs: func(temporal *mockgameflow.MockTemporal) {
				workflowRun := newWorkflowRun(workflowRunID)

				temporal.EXPECT().
					ExecuteWorkflow(gomock.Any(), gomock.Eq(workflowOpts), gomock.Any(), gomock.Eq(gameWorkflowInput)).
					Times(1).
					Return(workflowRun, nil)

				gameStateView := types.GameStateView{
					GameID: gameID,
					Phase:  types.PhaseEndGame,
				}

				encodedValue := newEncodedValue(&gameStateView, nil)

				temporal.EXPECT().
					QueryWorkflow(gomock.Any(), gomock.Eq(gameServerID.String()), gomock.Eq(""), gomock.Eq(workflow.QueryGetGameState)).
					Times(1).
					Return(encodedValue, nil)
			},
			checkResponse: func(t *testing.T, outputGameID uuid.UUID, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrGameEnding)
				require.Equal(t, gameID, outputGameID)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			temporal, client := createTestClient(t)
			require.NotNil(t, client)
			require.NotEmpty(t, temporal)

			tc.buildStubs(temporal)

			gameID, err := client.Game(gameServerID, gameWorkflowInput)
			tc.checkResponse(t, gameID, err)
		})
	}
}

func TestGetGameState(t *testing.T) {
	gameID := uuid.New()
	gameServerID := uuid.New()

	expectedGameStateView := types.GameStateView{
		GameID: gameID,
		Phase:  types.PhaseInRound,
	}

	testCases := []struct {
		name          string
		buildStubs    func(temporal *mockgameflow.MockTemporal)
		checkResponse func(t *testing.T, gameStateView *types.GameStateView, err error)
	}{
		{
			name: "success",
			buildStubs: func(temporal *mockgameflow.MockTemporal) {
				encodedValue := newEncodedValue(&expectedGameStateView, nil)

				temporal.EXPECT().
					QueryWorkflow(gomock.Any(), gomock.Eq(gameServerID.String()), gomock.Eq(""), gomock.Eq(workflow.QueryGetGameState)).
					Times(1).
					Return(encodedValue, nil)
			},
			checkResponse: func(t *testing.T, gameStateView *types.GameStateView, err error) {
				require.NoError(t, err)
				require.Equal(t, expectedGameStateView, *gameStateView)
			},
		},
		{
			name: "could not decode game state",
			buildStubs: func(temporal *mockgameflow.MockTemporal) {
				encodedValue := newEncodedValue(nil, errors.New("error decoding game state"))

				temporal.EXPECT().
					QueryWorkflow(gomock.Any(), gomock.Eq(gameServerID.String()), gomock.Eq(""), gomock.Eq(workflow.QueryGetGameState)).
					Times(1).
					Return(encodedValue, nil)
			},
			checkResponse: func(t *testing.T, gameStateView *types.GameStateView, err error) {
				require.Error(t, err)
				require.Nil(t, gameStateView)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			temporal, client := createTestClient(t)
			require.NotNil(t, client)
			require.NotEmpty(t, temporal)

			tc.buildStubs(temporal)

			gameStateView, err := client.GetGameState(gameServerID)
			tc.checkResponse(t, gameStateView, err)
		})
	}
}

func TestAddPlayers(t *testing.T) {
	t.Run("signals workflow on flush", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameID := uuid.New()
		gameServerID := uuid.New()
		playerIDs := []uuid.UUID{uuid.New(), uuid.New()}
		playersJoinSignal := workflow.PlayersJoinSignal{
			GameID:    gameID,
			PlayerIDs: playerIDs,
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalPlayersJoin),
				gomock.Eq(playersJoinSignal),
			).
			Return(nil)

		client.AddPlayers(gameServerID, gameID, playerIDs)
		client.Flush()
	})

	t.Run("signals workflow on game server shutdown", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameID := uuid.New()
		gameServerID := uuid.New()
		playerIDs := []uuid.UUID{uuid.New(), uuid.New()}
		playersJoinSignal := workflow.PlayersJoinSignal{
			GameID:    gameID,
			PlayerIDs: playerIDs,
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalPlayersJoin),
				gomock.Eq(playersJoinSignal),
			).
			Return(nil)

		client.AddPlayers(gameServerID, gameID, playerIDs)
		client.ShutdownGameServer(gameServerID)
	})

	t.Run("skips signalling workflow if no players to add", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameID := uuid.New()
		gameServerID := uuid.New()
		var playerIDs []uuid.UUID

		client.AddPlayers(gameServerID, gameID, playerIDs)
		client.Flush()
	})

	t.Run("retries after transient failure", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameID := uuid.New()
		gameServerID := uuid.New()
		playerIDs := []uuid.UUID{uuid.New(), uuid.New()}
		playersJoinSignal := workflow.PlayersJoinSignal{
			GameID:    gameID,
			PlayerIDs: playerIDs,
		}

		gomock.InOrder(
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalPlayersJoin),
					gomock.Eq(playersJoinSignal),
				).
				Return(errors.New("error signaling workflow")),
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalPlayersJoin),
					gomock.Eq(playersJoinSignal),
				).
				Return(nil),
		)

		client.AddPlayers(gameServerID, gameID, playerIDs)
		client.Flush()                          // Fails, re-buffers
		client.ShutdownGameServer(gameServerID) // Retries, succeeds
	})
}

func TestRemovePlayers(t *testing.T) {
	t.Run("signals workflow on flush", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		playerIDs := []uuid.UUID{uuid.New(), uuid.New()}
		signal := workflow.PlayersLeaveSignal{
			PlayerIDs: playerIDs,
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalPlayersLeave),
				gomock.Eq(signal),
			).
			Return(nil)

		client.RemovePlayers(gameServerID, playerIDs)
		client.Flush()
	})

	t.Run("signals workflow on game server shutdown", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		playerIDs := []uuid.UUID{uuid.New(), uuid.New()}
		signal := workflow.PlayersLeaveSignal{
			PlayerIDs: playerIDs,
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalPlayersLeave),
				gomock.Eq(signal),
			).
			Return(nil)

		client.RemovePlayers(gameServerID, playerIDs)
		client.ShutdownGameServer(gameServerID)
	})

	t.Run("skips signalling workflow if no players to remove", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		var playerIDs []uuid.UUID

		client.RemovePlayers(gameServerID, playerIDs)
		client.Flush()
	})

	t.Run("retries after transient failure", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		playerIDs := []uuid.UUID{uuid.New(), uuid.New()}
		signal := workflow.PlayersLeaveSignal{
			PlayerIDs: playerIDs,
		}

		gomock.InOrder(
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalPlayersLeave),
					gomock.Eq(signal),
				).
				Return(errors.New("error signaling workflow")),
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalPlayersLeave),
					gomock.Eq(signal),
				).
				Return(nil),
		)

		client.RemovePlayers(gameServerID, playerIDs)
		client.Flush()                          // Fails, re-buffers
		client.ShutdownGameServer(gameServerID) // Retries, succeeds
	})
}

func TestReportPlayer(t *testing.T) {
	t.Run("signals workflow on flush", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		reporter := uuid.New()
		reported := uuid.New()
		reports := []types.PlayerReport{
			{
				ReportedID: reported,
				ReporterID: reporter,
			},
		}
		signal := workflow.PlayersReportedSignal{
			Reports: reports,
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalPlayersReported),
				gomock.Eq(signal),
			).
			Return(nil)

		client.ReportPlayer(gameServerID, reporter, reported)
		client.Flush()
	})

	t.Run("signals workflow on game server shutdown", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		reporter := uuid.New()
		reported := uuid.New()
		reports := []types.PlayerReport{
			{
				ReportedID: reported,
				ReporterID: reporter,
			},
		}
		signal := workflow.PlayersReportedSignal{
			Reports: reports,
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalPlayersReported),
				gomock.Eq(signal),
			).
			Return(nil)

		client.ReportPlayer(gameServerID, reporter, reported)
		client.ShutdownGameServer(gameServerID)
	})

	t.Run("retries after transient failure", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		reporter := uuid.New()
		reported := uuid.New()
		reports := []types.PlayerReport{
			{
				ReportedID: reported,
				ReporterID: reporter,
			},
		}
		signal := workflow.PlayersReportedSignal{
			Reports: reports,
		}

		gomock.InOrder(
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalPlayersReported),
					gomock.Eq(signal),
				).
				Return(errors.New("error signaling workflow")),
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalPlayersReported),
					gomock.Eq(signal),
				).
				Return(nil),
		)

		client.ReportPlayer(gameServerID, reporter, reported)
		client.Flush()                          // Fails, re-buffers
		client.ShutdownGameServer(gameServerID) // Retries, succeeds
	})
}

func TestClearPlayerReports(t *testing.T) {
	t.Run("signals workflow on flush", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		playerID := uuid.New()
		signal := workflow.ClearPlayerReportsSignal{
			PlayerIDs: []uuid.UUID{playerID},
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalClearPlayerReports),
				gomock.Eq(signal),
			).
			Return(nil)

		client.ClearPlayerReports(gameServerID, playerID)
		client.Flush()
	})

	t.Run("signals workflow on game server shutdown", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		playerID := uuid.New()
		signal := workflow.ClearPlayerReportsSignal{
			PlayerIDs: []uuid.UUID{playerID},
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalClearPlayerReports),
				gomock.Eq(signal),
			).
			Return(nil)

		client.ClearPlayerReports(gameServerID, playerID)
		client.ShutdownGameServer(gameServerID)
	})

	t.Run("retries after transient failure", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		playerID := uuid.New()
		signal := workflow.ClearPlayerReportsSignal{
			PlayerIDs: []uuid.UUID{playerID},
		}

		gomock.InOrder(
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalClearPlayerReports),
					gomock.Eq(signal),
				).
				Return(errors.New("error signaling workflow")),
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalClearPlayerReports),
					gomock.Eq(signal),
				).
				Return(nil),
		)

		client.ClearPlayerReports(gameServerID, playerID)
		client.Flush()                          // Fails, re-buffers
		client.ShutdownGameServer(gameServerID) // Retries, succeeds
	})
}

func TestEjectPlayer(t *testing.T) {
	t.Run("signals workflow on flush", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		playerID := uuid.New()
		ejectorID := uuid.New()
		ejections := []types.PlayerEjection{
			{
				PlayerID:  playerID,
				EjectorID: ejectorID,
			},
		}
		signal := workflow.PlayersEjectedSignal{
			Ejections: ejections,
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalPlayersEjected),
				gomock.Eq(signal),
			).
			Return(nil)

		client.EjectPlayer(gameServerID, playerID, ejectorID)
		client.Flush()
	})

	t.Run("signals workflow on game server shutdown", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		playerID := uuid.New()
		ejectorID := uuid.New()
		ejections := []types.PlayerEjection{
			{
				PlayerID:  playerID,
				EjectorID: ejectorID,
			},
		}
		signal := workflow.PlayersEjectedSignal{
			Ejections: ejections,
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalPlayersEjected),
				gomock.Eq(signal),
			).
			Return(nil)

		client.EjectPlayer(gameServerID, playerID, ejectorID)
		client.ShutdownGameServer(gameServerID)
	})

	t.Run("retries after transient failure", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()
		playerID := uuid.New()
		ejectorID := uuid.New()
		ejections := []types.PlayerEjection{
			{
				PlayerID:  playerID,
				EjectorID: ejectorID,
			},
		}
		signal := workflow.PlayersEjectedSignal{
			Ejections: ejections,
		}

		gomock.InOrder(
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalPlayersEjected),
					gomock.Eq(signal),
				).
				Return(errors.New("error signaling workflow")),
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalPlayersEjected),
					gomock.Eq(signal),
				).
				Return(nil),
		)

		client.EjectPlayer(gameServerID, playerID, ejectorID)
		client.Flush()                          // Fails, re-buffers
		client.ShutdownGameServer(gameServerID) // Retries, succeeds
	})
}

func TestRecordCorrectGuesses(t *testing.T) {
	t.Run("signals workflow on flush", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameID := uuid.New()
		gameServerID := uuid.New()
		currentRound := uint8(1)
		correctGuesses := []types.CorrectGuess{
			{
				PlayerID:  uuid.New(),
				Timestamp: time.Now().UTC(),
			},
		}
		signal := workflow.CorrectGuessesSignal{
			GameID:       gameID,
			CurrentRound: currentRound,
			Guesses:      correctGuesses,
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalCorrectGuesses),
				gomock.Eq(signal),
			).
			Return(nil)

		client.RecordCorrectGuesses(gameServerID, gameID, currentRound, correctGuesses)
		client.Flush()
	})

	t.Run("signals workflow on game server shutdown", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameID := uuid.New()
		gameServerID := uuid.New()
		currentRound := uint8(1)
		correctGuesses := []types.CorrectGuess{
			{
				PlayerID:  uuid.New(),
				Timestamp: time.Now().UTC(),
			},
		}
		signal := workflow.CorrectGuessesSignal{
			GameID:       gameID,
			CurrentRound: currentRound,
			Guesses:      correctGuesses,
		}

		temporal.EXPECT().
			SignalWorkflow(
				gomock.Any(),
				gomock.Eq(gameServerID.String()),
				gomock.Eq(""),
				gomock.Eq(workflow.SignalCorrectGuesses),
				gomock.Eq(signal),
			).
			Return(nil)

		client.RecordCorrectGuesses(gameServerID, gameID, currentRound, correctGuesses)
		client.ShutdownGameServer(gameServerID)
	})

	t.Run("skips signalling workflow if no guesses to record", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameServerID := uuid.New()

		client.RecordCorrectGuesses(gameServerID, uuid.New(), uint8(1), []types.CorrectGuess{})
		client.ShutdownGameServer(gameServerID)
	})

	t.Run("retries after transient failure", func(t *testing.T) {
		temporal, client := createTestClient(t)
		require.NotNil(t, client)
		require.NotEmpty(t, temporal)

		gameID := uuid.New()
		gameServerID := uuid.New()
		currentRound := uint8(1)
		correctGuesses := []types.CorrectGuess{
			{
				PlayerID:  uuid.New(),
				Timestamp: time.Now().UTC(),
			},
		}
		signal := workflow.CorrectGuessesSignal{
			GameID:       gameID,
			CurrentRound: currentRound,
			Guesses:      correctGuesses,
		}

		gomock.InOrder(
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalCorrectGuesses),
					gomock.Eq(signal),
				).
				Return(errors.New("error signaling workflow")),
			temporal.EXPECT().
				SignalWorkflow(
					gomock.Any(),
					gomock.Eq(gameServerID.String()),
					gomock.Eq(""),
					gomock.Eq(workflow.SignalCorrectGuesses),
					gomock.Eq(signal),
				).
				Return(nil),
		)

		client.RecordCorrectGuesses(gameServerID, gameID, currentRound, correctGuesses)
		client.Flush()                          // Fails, re-buffers
		client.ShutdownGameServer(gameServerID) // Retries, succeeds
	})
}

func TestSelectWord(t *testing.T) {
	temporal, client := createTestClient(t)
	require.NotNil(t, client)
	require.NotEmpty(t, temporal)

	gameID := uuid.New()
	gameServerID := uuid.New()
	currentRound := uint8(1)
	word := "correct word"

	signal := workflow.WordSelectedSignal{
		GameID:       gameID,
		CurrentRound: currentRound,
		Word:         word,
	}

	signalErr := errors.New("error signaling workflow")
	temporal.EXPECT().
		SignalWorkflow(
			gomock.Any(),
			gomock.Eq(gameServerID.String()),
			gomock.Eq(""),
			gomock.Eq(workflow.SignalWordSelected),
			gomock.Eq(signal),
		).
		Return(signalErr)

	err := client.SelectWord(gameServerID, gameID, currentRound, word)
	require.ErrorIs(t, err, signalErr)
}

func TestArtistDisconnected(t *testing.T) {
	temporal, client := createTestClient(t)
	require.NotNil(t, client)
	require.NotEmpty(t, temporal)

	gameID := uuid.New()
	gameServerID := uuid.New()
	currentRound := uint8(1)
	artistID := uuid.New()

	signal := workflow.ArtistDisconnectedSignal{
		GameID:       gameID,
		CurrentRound: currentRound,
		ArtistID:     artistID,
	}

	signalErr := errors.New("error signaling workflow")
	temporal.EXPECT().
		SignalWorkflow(
			gomock.Any(),
			gomock.Eq(gameServerID.String()),
			gomock.Eq(""),
			gomock.Eq(workflow.SignalArtistDisconnected),
			gomock.Eq(signal),
		).
		Return(signalErr)

	err := client.ArtistDisconnected(gameServerID, gameID, currentRound, artistID)
	require.ErrorIs(t, err, signalErr)
}

func TestHeartbeatGameServerInstance(t *testing.T) {
	temporal, client := createTestClient(t)
	require.NotNil(t, client)
	require.NotEmpty(t, temporal)

	gameID := uuid.New()
	gameServerID := uuid.New()
	gameServerInstanceID := uuid.New()

	signal := workflow.GameServerInstanceHeartbeatSignal{
		InstanceID: gameServerInstanceID,
	}

	signalErr := errors.New("error signaling workflow")
	temporal.EXPECT().
		SignalWorkflow(
			gomock.Any(),
			gomock.Eq(gameServerID.String()),
			gomock.Eq(""),
			gomock.Eq(workflow.SignalGameServerInstanceHeartbeat),
			gomock.Eq(signal),
		).
		Return(signalErr)

	err := client.HeartbeatGameServerInstance(gameServerID, gameID, gameServerInstanceID)
	require.ErrorIs(t, err, signalErr)
}

func TestUnregisterGameServerInstance(t *testing.T) {
	temporal, client := createTestClient(t)
	require.NotNil(t, client)
	require.NotEmpty(t, temporal)

	gameServerID := uuid.New()
	gameServerInstanceID := uuid.New()

	signal := workflow.GameServerInstanceUnregisteredSignal{
		InstanceID: gameServerInstanceID,
	}

	signalErr := errors.New("error signaling workflow")
	temporal.EXPECT().
		SignalWorkflow(
			gomock.Any(),
			gomock.Eq(gameServerID.String()),
			gomock.Eq(""),
			gomock.Eq(workflow.SignalGameServerInstanceUnregistered),
			gomock.Eq(signal),
		).
		Return(signalErr)

	err := client.UnregisterGameServerInstance(gameServerID, gameServerInstanceID)
	require.ErrorIs(t, err, signalErr)
}

func TestAcknowledgeGameEvent(t *testing.T) {
	temporal, client := createTestClient(t)
	require.NotNil(t, client)
	require.NotEmpty(t, temporal)

	gameServerID := uuid.New()
	payload := gameevents.EventAckPayload{
		CorrelationID: uuid.New(),
		InstanceID:    uuid.New(),
		Status:        gameevents.AckStatusDelivered,
		Reason:        "success",
	}

	signalErr := errors.New("error signaling workflow")
	temporal.EXPECT().
		SignalWorkflow(
			gomock.Any(),
			gomock.Eq(gameServerID.String()),
			gomock.Eq(""),
			gomock.Eq(workflow.SignalEventAck),
			gomock.Eq(payload),
		).
		Return(signalErr)

	err := client.AcknowledgeGameEvent(gameServerID, payload)
	require.ErrorIs(t, err, signalErr)
}
