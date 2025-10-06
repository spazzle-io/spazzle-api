package websocketserver

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

const ServerJoinEndpoint = "/ws/servers/join"

var (
	ErrInternalServerError        = errors.New("an unexpected error occurred while processing your request")
	ErrMissingAuthorizationHeader = errors.New("missing authorization header")
	ErrInvalidAuthorizationHeader = errors.New("invalid authorization header format")
	ErrMissingUserID              = errors.New("missing user ID")
	ErrMissingServerID            = errors.New("missing server ID")
)

func checkOrigin(userId uuid.UUID, config util.Config) func(r *http.Request) bool {
	return func(r *http.Request) (isValid bool) {
		origin := r.Header.Get("Origin")
		defer func() {
			log.Debug().
				Str("origin", origin).
				Str("user_id", userId.String()).
				Bool("is_valid", isValid).
				Bool("in_dev_env", config.IsDevelopmentEnvironment()).
				Msg("validated ws upgrade request origin")
		}()
		if origin == "" {
			return true
		}

		if config.IsDevelopmentEnvironment() {
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

			if parsedAllowedOrigin.Scheme != "" && parsedAllowedOrigin.Scheme != parsedOrigin.Scheme {
				continue
			}

			if strings.HasSuffix(parsedOrigin.Hostname(), parsedAllowedOrigin.Hostname()) {
				return true
			}
		}

		return false
	}
}

func ServeWs(ctx context.Context, sm *GameServerManager, config util.Config, w http.ResponseWriter, r *http.Request) {
	userId, serverId, _, err := extractRequestMetadata(w, r)
	if err != nil {
		log.Error().Err(err).Msg("failed to extract metadata from ws upgrade request")
		return
	}

	logger := log.With().Str("user_id", userId.String()).Str("server_id", serverId.String()).Logger()

	// TODO: Validate the server join code

	gameServer := sm.GetOrCreateGameServer(ctx, serverId)
	if gameServer.IsClosed() {
		logger.Warn().Msg("game server already closed. attempting to create a new game server")
		sm.RemoveGameServerIfClosed(serverId)
		gameServer = sm.GetOrCreateGameServer(ctx, serverId)
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     checkOrigin(userId, config),
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error().Err(err).Msg("failed to upgrade HTTP server connection to websocket protocol")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return
	}

	client, err := NewClient(gameServer, conn, userId)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create ws client")
		if err = conn.Close(); err != nil {
			logger.Error().Err(err).Msg("failed to close client ws connection")
		}
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return
	}

	if err := gameServer.Register(client); err != nil {
		logger.Error().Err(err).Msg("failed to register client to game server")
		if err = conn.Close(); err != nil {
			logger.Error().Err(err).Msg("failed to close client ws connection")
		}
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return
	}

	go client.readPump(ctx)
	go client.writePump(ctx)
}

func extractRequestMetadata(
	w http.ResponseWriter,
	r *http.Request,
) (userId uuid.UUID, serverId uuid.UUID, serverJoinCode string, err error) {
	userIdStr := r.URL.Query().Get("user_id")
	serverIdStr := r.URL.Query().Get("server_id")

	if userIdStr == "" {
		log.Error().Msg("missing user ID in ws upgrade request")
		http.Error(w, ErrMissingUserID.Error(), http.StatusBadRequest)
		err = ErrMissingUserID
		return
	}

	logger := log.With().Str("user_id", userIdStr).Logger()

	userId, err = uuid.Parse(userIdStr)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse user ID from ws upgrade request")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return
	}

	if serverIdStr == "" {
		logger.Error().Msg("missing server ID in ws upgrade request")
		http.Error(w, ErrMissingServerID.Error(), http.StatusBadRequest)
		err = ErrMissingServerID
		return
	}

	logger = logger.With().Str("server_id", serverIdStr).Logger()

	serverId, err = uuid.Parse(serverIdStr)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse server ID from ws upgrade request")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return
	}

	authHeader := r.Header.Get(commonMiddleware.AuthorizationHeader)
	if authHeader == "" {
		logger.Error().Msg("missing authorization header in ws upgrade request")
		http.Error(w, ErrMissingAuthorizationHeader.Error(), http.StatusUnauthorized)
		err = ErrMissingAuthorizationHeader
		return
	}

	if !strings.HasPrefix(strings.ToLower(authHeader), commonMiddleware.AuthorizationBearer) {
		logger.Error().Msg("invalid authorization header in ws upgrade request")
		http.Error(w, ErrInvalidAuthorizationHeader.Error(), http.StatusUnauthorized)
		err = ErrInvalidAuthorizationHeader
		return
	}

	serverJoinCode = strings.TrimSpace(authHeader[len(commonMiddleware.AuthorizationBearer):])

	return
}
