package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/activities"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

func handlePhaseInRound(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info("entering in-round phase")

	state.SubPhase = types.SubPhaseConfirmArtist

	for {
		switch state.SubPhase {
		case types.SubPhaseConfirmArtist:
			handleSubPhaseConfirmArtist(ctx, state, notifyCh, logger)
		case types.SubPhaseWordSelection:
			handleSubPhaseWordSelection(ctx, state, notifyCh, logger)
		}

		if state.Phase != types.PhaseInRound {
			state.SubPhase = types.SubPhaseNone
			return
		}
	}
}

func handleSubPhaseConfirmArtist(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info("entering confirm artist sub-phase")

	artistConfirmedPayload, err := json.Marshal(&gameevents.ArtistConfirmedPayload{
		ArtistID:    state.CurrentArtist,
		RoundNumber: state.RoundNumber,
	})
	if err != nil {
		logger.Error("failed to marshal artist confirmed payload", "error", err)
		return
	}

	_, err = sendGameEvent(
		ctx, state, notifyCh, gameevents.TypeArtistConfirmed, artistConfirmedPayload, uuid.Nil, false,
	)
	if err != nil {
		logger.Error("failed to send artist confirmed event", "error", err)
		return
	}

	logger.Info("artist confirmed", "artist_id", state.CurrentArtist)
	state.SubPhase = types.SubPhaseWordSelection
}

func handleSubPhaseWordSelection(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info("entering word selection sub-phase")

	if err := sendBeginWordSelectionEvent(ctx, state, notifyCh); err != nil {
		logger.Error("failed to send begin word selection event", "error", err)
		return
	}

	selectWord(ctx, state, logger)
	if state.CurrentWord.Text == "" {
		logger.Error("failed to select word")
		return
	}

	err := sendWordSelectedEvent(ctx, state, notifyCh)
	if err != nil {
		logger.Error("failed to send word selected event", "error", err)
		return
	}

	logger.Info("word selected", "word", state.CurrentWord.Text)
	state.Phase = types.PhaseEndRound
}

func sendBeginWordSelectionEvent(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) error {
	endsAt := workflow.Now(ctx).UTC().Add(types.WordSelectionTimeout)

	selectionID, err := generateUUID(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate selection ID: %w", err)
	}

	state.SelectionID = selectionID

	beginWordSelectionPayload, err := json.Marshal(&gameevents.BeginWordSelectionPayload{
		ArtistID:    state.CurrentArtist,
		RoundNumber: state.RoundNumber,
		EndsAt:      endsAt,
		SelectionID: selectionID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal begin word selection payload: %w", err)
	}

	_, err = sendGameEvent(
		ctx, state, notifyCh, gameevents.TypeBeginWordSelection, beginWordSelectionPayload, uuid.Nil, false,
	)
	return err
}

func sendWordSelectedEvent(ctx workflow.Context, state *GameState, notifyCh workflow.Channel) error {
	wordSelectedPayload, err := json.Marshal(&gameevents.WordSelectedPayload{
		RoundNumber: state.RoundNumber,
		WordTokens:  toWordTokens(state.CurrentWord.Tokens),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal word selected payload: %w", err)
	}

	_, err = sendGameEvent(
		ctx, state, notifyCh, gameevents.TypeWordSelected, wordSelectedPayload, uuid.Nil, false,
	)
	return err
}

func selectWord(ctx workflow.Context, state *GameState, logger log.Logger) {
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	defer cancelTimer()

	var wordSignal WordSelectedSignal
	signalCh := workflow.GetSignalChannel(ctx, SignalWordSelected)

	selector := workflow.NewSelector(ctx)

	selector.AddReceive(signalCh, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &wordSignal)
		if wordSignal.SelectionID == state.SelectionID {
			state.CurrentWord = types.Word{
				Text:   wordSignal.Word,
				Tokens: getWordTokens(wordSignal.Word),
			}
		}
	})

	selector.AddFuture(workflow.NewTimer(timerCtx, types.WordSelectionTimeout), func(f workflow.Future) {
		logger.Warn("word selection timeout")

		word, err := selectRandomWord(ctx)
		if err != nil {
			logger.Error("failed to select random word", "error", err)
			return
		}

		state.CurrentWord = types.Word{
			Text:   word,
			Tokens: getWordTokens(word),
		}
	})

	selector.Select(ctx)
}

func selectRandomWord(ctx workflow.Context) (string, error) {
	var a *activities.Activities

	selectRandomWordParams := activities.SelectRandomWordParams{
		GameServerID: getGameServerID(ctx),
	}

	var selectRandomWordResult activities.SelectRandomWordResult
	err := workflow.ExecuteActivity(ctx, a.SelectRandomWord, selectRandomWordParams).Get(ctx, &selectRandomWordResult)

	return selectRandomWordResult.Word, err
}

func getWordTokens(word string) []types.Token {
	parts := strings.Fields(word)
	tokens := make([]types.Token, len(parts))

	for i, part := range parts {
		tokens[i] = types.Token{
			Length: utf8.RuneCountInString(part),
		}
	}

	return tokens
}

func toWordTokens(tokens []types.Token) []gameevents.WordToken {
	wordTokens := make([]gameevents.WordToken, len(tokens))

	for i, token := range tokens {
		wordTokens[i] = gameevents.WordToken{
			Length: token.Length,
		}
	}

	return wordTokens
}
