package websocketserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"

	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

const (
	WsServerJoinEndpoint      = "/ws/servers/join"
	serverJoinCodeCachePrefix = "server-join-code"
	ServerJoinCodeCacheTTL    = 15 * time.Minute
	nonceLength               = 8
)

var (
	ErrInternalServerError        = errors.New("an unexpected error occurred while processing your request")
	ErrMissingAuthorizationHeader = errors.New("missing authorization header")
	ErrInvalidAuthorizationHeader = errors.New("invalid authorization header format")
	ErrMissingUserID              = errors.New("missing user ID")
	ErrMissingServerID            = errors.New("missing server ID")
	ErrUnauthorizedAccess         = errors.New("authorization failed. Please verify your credentials and try again")
)

// ServeWsOptions defines optional behaviors for the ServeWs function.
// It allows callers to customize how the WebSocket connection is initialized.
type ServeWsOptions struct {
	// When true (default), ServeWs will automatically start the client's
	// read and write goroutines. This configuration is primarily intended for unit testing.
	StartPumps bool
}

func ServeWs(
	ctx context.Context,
	sm *gameserver.Manager,
	config util.Config,
	cache commonCache.Cache,
	w http.ResponseWriter,
	r *http.Request,
	opts *ServeWsOptions,
) (*gameserver.Client, error) {
	if opts == nil {
		opts = &ServeWsOptions{
			StartPumps: true,
		}
	}

	userId, serverId, serverJoinCode, err := extractRequestMetadata(w, r)
	if err != nil {
		log.Error().Err(err).Msg("failed to extract metadata from ws upgrade request")
		return nil, err
	}

	logger := log.With().Str("user_id", userId.String()).Str("server_id", serverId.String()).Logger()

	validServerJoinCode, err := fetchServerJoinCode(ctx, w, userId, serverId, config, cache)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch server join code")
		return nil, err
	}
	if !strings.EqualFold(validServerJoinCode, serverJoinCode) {
		logger.Error().Msg("server join code mismatch")
		http.Error(w, ErrUnauthorizedAccess.Error(), http.StatusUnauthorized)
		return nil, ErrUnauthorizedAccess
	}

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
		return nil, err
	}

	clientConns := gameServer.GetClientConnections(userId)
	isClientSpectating := len(clientConns) > 0

	client, err := gameserver.NewClient(ctx, gameServer, conn, userId, isClientSpectating, opts.StartPumps)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create ws client")
		if err = conn.Close(); err != nil {
			logger.Error().Err(err).Msg("failed to close client ws connection")
		}
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return client, err
	}

	if err := gameServer.Register(client); err != nil {
		logger.Error().Err(err).Msg("failed to register client to game server")
		if err = conn.Close(); err != nil {
			logger.Error().Err(err).Msg("failed to close client ws connection")
		}
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return client, err
	}

	return client, nil
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
		err = ErrInternalServerError
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
		err = ErrInternalServerError
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

func getServerJoinCodeCacheKey(userId uuid.UUID, serverId uuid.UUID, config util.Config) string {
	return fmt.Sprintf("%s-%s:%s:%s",
		config.ServiceName,
		serverJoinCodeCachePrefix,
		serverId.String(),
		userId.String(),
	)
}

func GenerateServerJoinCode(
	ctx context.Context,
	userId uuid.UUID,
	serverId uuid.UUID,
	config util.Config,
	cache commonCache.Cache,
) (string, error) {
	joinCode, err := commonUtil.GenerateRandomAlphanumericString(nonceLength)
	if err != nil {
		return "", fmt.Errorf("could not generate server join code: %w", err)
	}

	cacheKey := getServerJoinCodeCacheKey(userId, serverId, config)
	err = cache.Set(ctx, cacheKey, joinCode, ServerJoinCodeCacheTTL)
	if err != nil {
		return "", fmt.Errorf("could not cache server join code: %w", err)
	}

	return joinCode, nil
}

func fetchServerJoinCode(
	ctx context.Context,
	w http.ResponseWriter,
	userId uuid.UUID,
	serverId uuid.UUID,
	config util.Config,
	cache commonCache.Cache,
) (string, error) {
	logger := log.With().Str("user_id", userId.String()).Str("server_id", serverId.String()).Logger()

	cacheKey := getServerJoinCodeCacheKey(userId, serverId, config)
	res, err := cache.Get(ctx, cacheKey)
	if err != nil {
		logger.Error().Err(err).Msg("could not fetch server join code from cache")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return "", ErrInternalServerError
	}

	if res == nil {
		logger.Error().Msg("server join code not found in cache")
		http.Error(w, ErrUnauthorizedAccess.Error(), http.StatusUnauthorized)
		return "", ErrUnauthorizedAccess
	}

	joinCode, ok := res.(string)
	if !ok {
		logger.Error().Msg("could not cast server join code to string")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return "", ErrInternalServerError
	}

	err = cache.Del(ctx, cacheKey)
	if err != nil {
		logger.Error().Err(err).Msg("could not delete server join code from cache")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return "", ErrInternalServerError
	}

	return joinCode, nil
}
