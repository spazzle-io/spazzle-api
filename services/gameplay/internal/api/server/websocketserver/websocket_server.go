package websocketserver

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/infra"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

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

func ServeWs(
	ctx context.Context,
	res *infra.Resources,
	w http.ResponseWriter,
	r *http.Request,
) (*gameserver.Client, error) {
	joinCode, err := extractJoinCode(w, r)
	if err != nil {
		log.Error().Err(err).Msg("failed to extract join code from request")
		return nil, err
	}

	joinCodeEntry, err := res.GameCache.GetJoinCodeEntry(ctx, joinCode)
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
		Env:       res.Config,
		Store:     res.Store,
		Cache:     res.Cache,
		GameCache: res.GameCache,
		Bus:       res.Bus,
		GfClient:  res.GfClient,
		WordStore: res.WordStore,
	}

	gameServer, err := res.GsManager.GetOrCreateGameServer(joinCodeEntry.ServerID, gameServerConfig)
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

	_, err = res.GameCache.ValidateJoinCode(ctx, joinCode, gameServer.GetServerId(), gameServer.GetGameID())
	if err != nil {
		logger.Error().Err(err).Msg("failed to validate join code")
		http.Error(w, ErrUnauthorizedAccess.Error(), http.StatusUnauthorized)
		return nil, err
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     checkOrigin(res.Config, joinCodeEntry.UserID),
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

	err = res.GameCache.InvalidateJoinCode(ctx, joinCode)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to invalidate join code")
	}

	return client, nil
}

func extractJoinCode(w http.ResponseWriter, r *http.Request) (string, error) {
	joinCode := r.URL.Query().Get("join_code")
	if joinCode == "" {
		http.Error(w, ErrMissingJoinCode.Error(), http.StatusBadRequest)
		return "", ErrMissingJoinCode
	}

	return joinCode, nil
}

func checkOrigin(config *util.Config, userID uuid.UUID) func(r *http.Request) bool {
	return func(r *http.Request) (isValid bool) {
		origin := r.Header.Get("Origin")
		defer func() {
			log.Debug().
				Str("origin", origin).
				Str("user_id", userID.String()).
				Bool("is_valid", isValid).
				Bool("in_dev_env", config.Is(commonConfig.Development)).
				Msg("validated ws upgrade request origin")
		}()
		if origin == "" {
			return true
		}

		if config.Is(commonConfig.Development) {
			return true
		}

		allowedOrigins := config.AllowedOrigins

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
