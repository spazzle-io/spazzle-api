package websocketserver

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/api/deps"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const WsServerJoinEndpoint = "/ws/servers/join"

var (
	ErrInternalServerError = errors.New("an unexpected error occurred while processing your request")
	ErrUnauthorizedAccess  = errors.New("authorization failed. Please verify your credentials and try again")
	ErrMissingJoinCode     = errors.New("missing join code")
)

type WsHandler struct {
	*deps.APIServerDeps
}

func (h *WsHandler) ServeWs(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
) (*gameserver.Client, error) {
	joinCode, err := h.extractJoinCode(w, r)
	if err != nil {
		log.Error().Err(err).Msg("failed to extract join code from request")
		return nil, err
	}

	joinCodeEntry, err := h.GameCache.GetJoinCodeEntry(ctx, joinCode)
	if err != nil {
		log.Error().Err(err).Msg("failed to get join code entry")
		http.Error(w, ErrUnauthorizedAccess.Error(), http.StatusUnauthorized)
		return nil, err
	}

	logger := log.With().
		Str("user_id", joinCodeEntry.UserID.String()).
		Str("server_id", joinCodeEntry.ServerID.String()).
		Logger()

	gameServerConfig := &gameserver.Config{
		Env:       h.Config,
		Store:     h.Store,
		Cache:     h.Cache,
		GameCache: h.GameCache,
		Bus:       h.Bus,
		GfClient:  h.GfClient,
		WordStore: h.WordStore,
	}

	gameServer, err := h.GsManager.GetOrCreateGameServer(joinCodeEntry.ServerID, gameServerConfig)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get or create game server instance")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return nil, err
	}

	if gameServer.IsClosed() {
		logger.Error().Msg("game server is closed")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return nil, ErrInternalServerError
	}

	if !gameServer.IsGameActive() {
		logger.Error().Msg("game server is not active")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return nil, ErrInternalServerError
	}

	_, err = h.GameCache.ValidateJoinCode(ctx, joinCode, gameServer.GetServerId(), gameServer.GetGameID())
	if err != nil {
		logger.Error().Err(err).Msg("failed to validate join code")
		http.Error(w, ErrUnauthorizedAccess.Error(), http.StatusUnauthorized)
		return nil, err
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     h.checkOrigin(joinCodeEntry.UserID),
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error().Err(err).Msg("failed to upgrade HTTP server connection to websocket protocol")
		return nil, err
	}

	role, err := gameserver.ParseRole(joinCodeEntry.Role)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse role")
		if err := conn.Close(); err != nil {
			logger.Error().Err(err).Msg("failed to close client ws connection")
		}
		return nil, err
	}

	client, err := gameserver.NewClient(ctx, gameServer, conn, joinCodeEntry.UserID, role)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create ws client")
		if err = conn.Close(); err != nil {
			logger.Error().Err(err).Msg("failed to close client ws connection")
		}
		return client, err
	}

	if err := gameServer.Register(client); err != nil {
		logger.Error().Err(err).Msg("failed to register client to game server")
		if err = conn.Close(); err != nil {
			logger.Error().Err(err).Msg("failed to close client ws connection")
		}
		return client, err
	}

	err = h.GameCache.InvalidateJoinCode(ctx, joinCode)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to invalidate join code")
	}

	return client, nil
}

func (h *WsHandler) extractJoinCode(w http.ResponseWriter, r *http.Request) (string, error) {
	joinCode := r.URL.Query().Get("join_code")
	if joinCode == "" {
		http.Error(w, ErrMissingJoinCode.Error(), http.StatusBadRequest)
		return "", ErrMissingJoinCode
	}

	return joinCode, nil
}

func (h *WsHandler) checkOrigin(userID uuid.UUID) func(r *http.Request) bool {
	return func(r *http.Request) (isValid bool) {
		origin := r.Header.Get("Origin")
		defer func() {
			log.Debug().
				Str("origin", origin).
				Str("user_id", userID.String()).
				Bool("is_valid", isValid).
				Bool("in_dev_env", h.Config.IsDevelopmentEnvironment()).
				Msg("validated ws upgrade request origin")
		}()
		if origin == "" {
			return true
		}

		if h.Config.IsDevelopmentEnvironment() {
			return true
		}

		allowedOrigins := h.Config.AllowedOrigins

		parsedOrigin, err := url.Parse(origin)
		if err != nil {
			return false
		}

		for _, allowedOrigin := range allowedOrigins {
			parsedAllowedOrigin, err := url.Parse(allowedOrigin)
			if err != nil {
				continue
			}

			if parsedAllowedOrigin.Scheme == "" {
				continue
			}

			if parsedAllowedOrigin.Scheme != parsedOrigin.Scheme {
				continue
			}

			hostname := strings.ToLower(parsedOrigin.Hostname())
			allowed := strings.ToLower(parsedAllowedOrigin.Hostname())

			if hostname == allowed || strings.HasSuffix(hostname, "."+allowed) {
				return true
			}
		}

		return false
	}
}
