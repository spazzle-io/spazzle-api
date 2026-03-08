package workflow

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/activities"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/types"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

const (
	nextArtistSelectionBuffer = 10 * time.Second
	hintRevealBuffer          = 10 * time.Second
)

func handlePhaseInRound(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	logger.Info("entering in-round phase")

	state.SubPhase = types.SubPhaseConfirmArtist

	for {
		var err error

		switch state.SubPhase {
		case types.SubPhaseConfirmArtist:
			err = handleSubPhaseConfirmArtist(ctx, state, notifyCh, logger)
		case types.SubPhaseWordSelection:
			err = handleSubPhaseWordSelection(ctx, state, notifyCh, logger)
		case types.SubPhaseDrawing:
			err = handleSubPhaseDrawing(ctx, state, notifyCh, logger)
		}

		if err != nil {
			logger.Warn("error occurred in the in-round phase", "error", err)

			if state.CurrentRound > DefaultRoundNumber {
				logger.Info("ending game due to in-round failure")
				state.Phase = types.PhaseEndRound
				return
			}
		}

		if state.IsTerminated {
			state.Phase = types.PhaseEndRound
			return
		}

		if state.Phase != types.PhaseInRound {
			state.SubPhase = types.SubPhaseNone
			return
		}

		if err = workflow.Sleep(ctx, phaseCooldownDuration); err != nil {
			logger.Warn("failed to cooldown after in-round phase attempt", "error", err)
		}
	}
}

func handleSubPhaseConfirmArtist(
	ctx workflow.Context,
	state *GameState,
	notifyCh workflow.Channel,
	logger log.Logger,
) error {
	logger.Info("entering confirm artist sub-phase")

	payload := gameevents.ArtistConfirmedPayload{
		ArtistID:     state.CurrentArtist,
		CurrentRound: state.CurrentRound,
	}
	_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypeArtistConfirmed, payload, nil)
	if err != nil {
		return fmt.Errorf("failed to send artist confirmed event: %w", err)
	}

	logger.Info("artist confirmed", "artist_id", state.CurrentArtist)
	state.SubPhase = types.SubPhaseWordSelection
	return nil
}

func handleSubPhaseWordSelection(
	ctx workflow.Context,
	state *GameState,
	notifyCh workflow.Channel,
	logger log.Logger,
) error {
	logger.Info("entering word selection sub-phase")

	if strings.TrimSpace(state.CurrentWord.Text) == "" {
		beginWordSelectionPayload := gameevents.BeginWordSelectionPayload{
			ArtistID:     state.CurrentArtist,
			CurrentRound: state.CurrentRound,
			EndsAt:       workflow.Now(ctx).UTC().Add(types.WordSelectionTimeout),
		}
		_, err := sendGameEvent(
			ctx, state, notifyCh, gameevents.TypeBeginWordSelection, beginWordSelectionPayload, nil,
		)
		if err != nil {
			return fmt.Errorf("failed to send begin word selection event: %w", err)
		}

		if !waitForWordSelection(ctx, state, logger) {
			return errors.New("failed to select word")
		}
	}

	wordSelectedPayload := gameevents.WordSelectedPayload{
		CurrentRound: state.CurrentRound,
		WordTokens:   toWordTokens(state.CurrentWord.Tokens),
	}
	_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypeWordSelected, wordSelectedPayload, nil)
	if err != nil {
		return fmt.Errorf("failed to send word selected event: %w", err)
	}

	logger.Info("word selected", "word", state.CurrentWord.Text)
	state.SubPhase = types.SubPhaseDrawing
	return nil
}

func handleSubPhaseDrawing(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) error {
	logger.Info("entering drawing sub-phase")

	beginDrawingPayload := gameevents.BeginDrawingPayload{
		CurrentRound: state.CurrentRound,
		EndsAt:       workflow.Now(ctx).UTC().Add(state.DrawingDuration),
	}
	_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypeBeginDrawing, beginDrawingPayload, nil)
	if err != nil {
		return fmt.Errorf("failed to send begin drawing event: %w", err)
	}

	drawingCtx, cancelDrawing := workflow.WithCancel(ctx)
	defer cancelDrawing()

	workflow.Go(drawingCtx, func(ctx workflow.Context) {
		handleCorrectGuesses(ctx, state, logger)
	})

	workflow.Go(drawingCtx, func(ctx workflow.Context) {
		scheduleNextArtistSelection(ctx, state, notifyCh, logger)
	})

	workflow.Go(drawingCtx, func(ctx workflow.Context) {
		scheduleHints(ctx, state, notifyCh, logger)
	})

	state.DrawingStartedAt = workflow.Now(ctx).UTC()
	timerFuture := workflow.NewTimer(drawingCtx, state.DrawingDuration)
	artistDisconnectedCh := workflow.GetSignalChannel(ctx, SignalArtistDisconnected)

	isDrawingComplete := false
	isArtistDisconnected := false

	selector := workflow.NewSelector(ctx)

	selector.AddFuture(timerFuture, func(f workflow.Future) {
		isDrawingComplete = true
	})

	selector.AddReceive(artistDisconnectedCh, func(c workflow.ReceiveChannel, more bool) {
		var signal ArtistDisconnectedSignal
		c.Receive(ctx, &signal)

		isArtistDisconnected = handleArtistDisconnectedSignal(ctx, state, signal, notifyCh, logger)
		if isArtistDisconnected {
			logger.Warn("artist disconnected during drawing phase")
		}
	})

	selector.AddReceive(notifyCh, func(c workflow.ReceiveChannel, more bool) {
		var tmp struct{}
		c.Receive(ctx, &tmp)

		if artist, exists := state.Players[state.CurrentArtist]; exists {
			if !artist.IsConnected || artist.IsEjected {
				isArtistDisconnected = true
				sendArtistDisconnectedEvent(ctx, state, notifyCh, logger)
			}
		}
	})

	for !isDrawingComplete && !isArtistDisconnected {
		selector.Select(ctx)
	}

	endDrawingPayload := gameevents.EndDrawingPayload{
		CurrentRound: state.CurrentRound,
		Word:         state.CurrentWord.Text,
	}
	_, err = sendGameEvent(ctx, state, notifyCh, gameevents.TypeEndDrawing, endDrawingPayload, nil)
	if err != nil {
		logger.Error("failed to send end drawing event", "error", err)
	}

	logger.Info("drawing phase complete")
	state.Phase = types.PhaseEndRound
	return nil
}

func handleArtistDisconnectedSignal(
	ctx workflow.Context,
	state *GameState,
	signal ArtistDisconnectedSignal,
	notifyCh workflow.Channel,
	logger log.Logger,
) (isDisconnected bool) {
	if signal.GameID != state.GameID || signal.CurrentRound != state.CurrentRound {
		return
	}
	if signal.ArtistID != state.CurrentArtist {
		return
	}

	if player, exists := state.Players[signal.ArtistID]; exists {
		player.IsConnected = false
		player.LeftAt = workflow.Now(ctx).UTC()
	}

	// TODO: Fine artist

	sendArtistDisconnectedEvent(ctx, state, notifyCh, logger)
	return true
}

func sendArtistDisconnectedEvent(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	payload := gameevents.ArtistDisconnectedPayload{
		CurrentRound: state.CurrentRound,
		ArtistID:     state.CurrentArtist,
	}
	_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypeArtistDisconnected, payload, nil)
	if err != nil {
		logger.Error("failed to send artist disconnected event", "error", err)
	}
}

func waitForWordSelection(ctx workflow.Context, state *GameState, logger log.Logger) (selected bool) {
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	defer cancelTimer()

	var wordSignal WordSelectedSignal
	signalCh := workflow.GetSignalChannel(ctx, SignalWordSelected)

	selector := workflow.NewSelector(ctx)

	selector.AddReceive(signalCh, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, &wordSignal)

		if wordSignal.GameID != state.GameID || wordSignal.CurrentRound != state.CurrentRound {
			return
		}

		state.CurrentWord = types.Word{
			Text:   strings.TrimSpace(wordSignal.Word),
			Tokens: getWordTokens(wordSignal.Word),
		}
		selected = true
	})

	selector.AddFuture(workflow.NewTimer(timerCtx, types.WordSelectionTimeout), func(f workflow.Future) {
		logger.Warn("word selection timeout")

		word, err := selectRandomWord(ctx)
		if err != nil {
			logger.Error("failed to select random word", "error", err)
			return
		}

		state.CurrentWord = types.Word{
			Text:   strings.TrimSpace(word),
			Tokens: getWordTokens(word),
		}
		selected = true
	})

	selector.Select(ctx)

	return selected
}

func selectRandomWord(ctx workflow.Context) (string, error) {
	var a *activities.Activities

	params := activities.SelectRandomWordParams{
		GameServerID: getGameServerID(ctx),
	}

	var result activities.SelectRandomWordResult
	err := workflow.ExecuteActivity(ctx, a.SelectRandomWord, params).Get(ctx, &result)

	return result.Word, err
}

func getWordTokens(word string) []types.Token {
	parts := strings.Fields(word)
	tokens := make([]types.Token, len(parts))

	for i, part := range parts {
		tokens[i] = types.Token{
			Text:   part,
			Length: commonUtil.GraphemeLen(part),
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

func handleCorrectGuesses(ctx workflow.Context, state *GameState, logger log.Logger) {
	state.CorrectGuesses[state.CurrentRound] = make([]types.CorrectGuess, 0)
	state.CorrectGuessers[state.CurrentRound] = make(map[uuid.UUID]bool)

	ch := workflow.GetSignalChannel(ctx, SignalCorrectGuesses)

	for {
		selector := workflow.NewSelector(ctx)
		selector.AddReceive(ch, func(c workflow.ReceiveChannel, more bool) {
			var signal CorrectGuessesSignal
			c.Receive(ctx, &signal)

			if signal.GameID != state.GameID || signal.CurrentRound != state.CurrentRound {
				return
			}

			for _, guess := range signal.Guesses {
				if state.CorrectGuessers[state.CurrentRound][guess.PlayerID] {
					continue
				}

				state.CorrectGuessers[state.CurrentRound][guess.PlayerID] = true
				state.CorrectGuesses[state.CurrentRound] = append(state.CorrectGuesses[state.CurrentRound], guess)
				logger.Info("correct guess recorded",
					"player_id", guess.PlayerID, "guessed_at", guess.Timestamp)
			}
		})

		selector.Select(ctx)

		if ctx.Err() != nil {
			return
		}
	}
}

func scheduleNextArtistSelection(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	if int32(state.CurrentRound) == state.NumRounds {
		logger.Info("last round. skipping next artist selection")
		return
	}

	encodedSelectionDelay := workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		selectionDelay, err := getNextArtistSelectionDelay(state)
		if err != nil {
			logger.Error("failed to get next artist selection delay", "error", err)
			return state.DrawingDuration / 2
		}

		return selectionDelay
	})

	var selectionDelay time.Duration
	err := encodedSelectionDelay.Get(&selectionDelay)
	if err != nil {
		logger.Error("failed to extract next artist selection delay", "error", err)
		selectionDelay = state.DrawingDuration / 2
	}

	if err := workflow.Sleep(ctx, selectionDelay); err != nil {
		return
	}

	nextArtist, err := selectArtist(ctx, state, logger)
	if err != nil {
		logger.Error("failed to select next artist", "error", err)
		return
	}

	payload := gameevents.NextArtistSelectedPayload{
		PlayerID: nextArtist,
		Round:    state.CurrentRound + 1,
	}
	delivered, err := sendGameEvent(
		ctx, state, notifyCh, gameevents.TypeNextArtistSelected, payload, &sendGameEventOpts{
			TargetClientID: nextArtist,
			WaitForAck:     true,
		},
	)
	if err != nil {
		logger.Error("failed to send next artist selected event", "error", err)
		return
	}
	if !delivered {
		logger.Warn("failed to deliver next artist selected event")
		if player, exists := state.Players[nextArtist]; exists {
			player.IsConnected = false
		}
		return
	}

	state.NextArtist = nextArtist
	logger.Info("next artist selected", "next_artist", nextArtist)
}

func getNextArtistSelectionDelay(state *GameState) (time.Duration, error) {
	if state.DrawingDuration <= 0 {
		return 0, nil
	}

	if state.DrawingDuration <= 2*nextArtistSelectionBuffer {
		n, err := rand.Int(rand.Reader, big.NewInt(state.DrawingDuration.Nanoseconds()))
		if err != nil {
			return 0, err
		}
		return time.Duration(n.Int64()), nil
	}

	availableSelectionDuration := state.DrawingDuration - 2*nextArtistSelectionBuffer
	n, err := rand.Int(rand.Reader, big.NewInt(availableSelectionDuration.Nanoseconds()))
	if err != nil {
		return 0, err
	}

	return nextArtistSelectionBuffer + time.Duration(n.Int64()), nil
}

func scheduleHints(ctx workflow.Context, state *GameState, notifyCh workflow.Channel, logger log.Logger) {
	if state.DrawingDuration <= 0 {
		return
	}

	encodedNumHints := workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		return getNumHintsForWord(state.CurrentWord, logger)
	})

	var numHints int
	err := encodedNumHints.Get(&numHints)
	if err != nil {
		logger.Error("failed to extract number of hints", "error", err)
		numHints = 1
	}

	encodedHints := workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		hints, err := getRandomHintsForWord(state.CurrentWord, numHints, logger)
		if err != nil {
			logger.Error("failed to get random hints for word", "error", err)
		}
		return hints
	})

	var hints []gameevents.WordHint
	err = encodedHints.Get(&hints)
	if err != nil {
		logger.Error("failed to extract hints", "error", err)
		return
	}

	if len(hints) == 0 {
		return
	}

	startOffset := hintRevealBuffer
	if state.DrawingDuration <= 2*hintRevealBuffer {
		startOffset = 0
	}

	availableDuration := state.DrawingDuration - 2*startOffset
	if availableDuration <= 0 {
		return
	}

	hintInterval := availableDuration / time.Duration(len(hints))

	if err := workflow.Sleep(ctx, startOffset); err != nil {
		return
	}

	for _, hint := range hints {
		if err := workflow.Sleep(ctx, hintInterval); err != nil {
			return
		}

		payload := gameevents.WordHintRevealedPayload{
			CurrentRound: state.CurrentRound,
			WordHint:     hint,
		}
		_, err := sendGameEvent(ctx, state, notifyCh, gameevents.TypeWordHintRevealed, payload, nil)
		if err != nil {
			logger.Error("failed to send word hint revealed event", "error", err)
		} else {
			logger.Info("word hint revealed",
				"token_idx", hint.WordTokenIdx, "char_idx", hint.CharIdx, "char", hint.Char)
		}
	}
}

func getMaxHintsForWord(word types.Word) int {
	totalChars := 0
	for _, token := range word.Tokens {
		totalChars += token.Length
	}

	if totalChars <= 4 {
		return 1
	}
	return 1 + (totalChars - 4)
}

func getNumHintsForWord(word types.Word, logger log.Logger) int {
	maxHints := getMaxHintsForWord(word)
	if maxHints <= 1 {
		return 1
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxHints)))
	if err != nil {
		logger.Error("failed to generate num hints for word", "error", err)
		return maxHints
	}

	return int(n.Int64()) + 1
}

func getRandomHintsForWord(word types.Word, numHints int, logger log.Logger) ([]gameevents.WordHint, error) {
	totalChars := 0
	for _, token := range word.Tokens {
		totalChars += token.Length
	}

	hintIndices, err := commonUtil.RandomIndices(totalChars, numHints)
	if err != nil {
		return nil, err
	}

	hints := make([]gameevents.WordHint, 0, numHints)

	for _, hintIdx := range hintIndices {
		charOffset := hintIdx
		for tokenIdx, token := range word.Tokens {
			if charOffset < token.Length {
				char, err := commonUtil.CharAt(token.Text, charOffset)
				if err != nil {
					logger.Warn("failed to find hint char", "error", err,
						"text", token.Text, "char", charOffset)
					break
				}

				hints = append(hints, gameevents.WordHint{
					WordTokenIdx: tokenIdx,
					CharIdx:      charOffset,
					Char:         char,
				})
				break
			}
			charOffset -= token.Length
		}
	}

	return hints, nil
}
