package websocketserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow"

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

type WsHandler struct {
	Config    util.Config
	Store     db.Store
	Cache     commonCache.Cache
	GfClient  gameflow.Client
	Bus       eventbus.EventBus
	GsManager *gameserver.Manager
	WordStore wordstore.Store
}

// ServeWsOptions defines optional behaviors for the ServeWs function.
// It allows callers to customize how the WebSocket connection is initialized.
type ServeWsOptions struct {
	// When true (default), ServeWs will automatically start the client's
	// read and write goroutines. This configuration is primarily intended for unit testing.
	StartPumps bool
}

func (h *WsHandler) ServeWs(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	opts *ServeWsOptions,
) (*gameserver.Client, error) {
	if opts == nil {
		opts = &ServeWsOptions{
			StartPumps: true,
		}
	}

	userID, serverID, serverJoinCode, err := h.extractRequestMetadata(w, r)
	if err != nil {
		log.Error().Err(err).Msg("failed to extract metadata from ws upgrade request")
		return nil, err
	}

	logger := log.With().Str("user_id", userID.String()).Str("server_id", serverID.String()).Logger()

	validServerJoinCode, err := fetchServerJoinCode(ctx, w, userID, serverID, h.Config, h.Cache)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch server join code")
		return nil, err
	}
	if validServerJoinCode != serverJoinCode {
		logger.Error().Msg("server join code mismatch")
		http.Error(w, ErrUnauthorizedAccess.Error(), http.StatusUnauthorized)
		return nil, ErrUnauthorizedAccess
	}

	gameServerConfig := &gameserver.Config{
		Env:       h.Config,
		Store:     h.Store,
		Cache:     h.Cache,
		Bus:       h.Bus,
		GfClient:  h.GfClient,
		WordStore: h.WordStore,
	}

	gameServer, err := h.GsManager.GetOrCreateGameServer(serverID, gameServerConfig)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get or create game server instance")
		return nil, ErrInternalServerError
	}

	if gameServer.IsClosed() {
		logger.Warn().Msg("game server already closed. attempting to create a new game server")
		h.GsManager.RemoveGameServerIfClosed(serverID)
		gameServer, err = h.GsManager.GetOrCreateGameServer(serverID, gameServerConfig)
		if err != nil {
			logger.Error().Err(err).Msg("failed to get or create game server instance")
			return nil, ErrInternalServerError
		}
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     h.checkOrigin(userID),
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error().Err(err).Msg("failed to upgrade HTTP server connection to websocket protocol")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return nil, err
	}

	clientConns := gameServer.GetClientConnections(userID)
	isClientSpectating := len(clientConns) > 0

	client, err := gameserver.NewClient(ctx, gameServer, conn, userID, isClientSpectating, opts.StartPumps)
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

func (h *WsHandler) extractRequestMetadata(
	w http.ResponseWriter,
	r *http.Request,
) (userID uuid.UUID, serverID uuid.UUID, serverJoinCode string, err error) {
	userIDStr := r.URL.Query().Get("user_id")
	serverIDStr := r.URL.Query().Get("server_id")

	if userIDStr == "" {
		log.Error().Msg("missing user ID in ws upgrade request")
		http.Error(w, ErrMissingUserID.Error(), http.StatusBadRequest)
		err = ErrMissingUserID
		return
	}

	logger := log.With().Str("user_id", userIDStr).Logger()

	userID, err = uuid.Parse(userIDStr)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse user ID from ws upgrade request")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		err = ErrInternalServerError
		return
	}

	if serverIDStr == "" {
		logger.Error().Msg("missing server ID in ws upgrade request")
		http.Error(w, ErrMissingServerID.Error(), http.StatusBadRequest)
		err = ErrMissingServerID
		return
	}

	logger = logger.With().Str("server_id", serverIDStr).Logger()

	serverID, err = uuid.Parse(serverIDStr)
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

func getServerJoinCodeCacheKey(userID uuid.UUID, serverID uuid.UUID, config util.Config) string {
	return fmt.Sprintf("%s-%s:%s:%s",
		config.ServiceName,
		serverJoinCodeCachePrefix,
		serverID.String(),
		userID.String(),
	)
}

func GenerateServerJoinCode(
	ctx context.Context,
	userID uuid.UUID,
	serverID uuid.UUID,
	config util.Config,
	cache commonCache.Cache,
) (string, error) {
	joinCode, err := commonUtil.GenerateRandomAlphanumericString(nonceLength)
	if err != nil {
		return "", fmt.Errorf("could not generate server join code: %w", err)
	}

	cacheKey := getServerJoinCodeCacheKey(userID, serverID, config)
	err = cache.Set(ctx, cacheKey, joinCode, ServerJoinCodeCacheTTL)
	if err != nil {
		return "", fmt.Errorf("could not cache server join code: %w", err)
	}

	return joinCode, nil
}

func fetchServerJoinCode(
	ctx context.Context,
	w http.ResponseWriter,
	userID uuid.UUID,
	serverID uuid.UUID,
	config util.Config,
	cache commonCache.Cache,
) (string, error) {
	logger := log.With().Str("user_id", userID.String()).Str("server_id", serverID.String()).Logger()

	cacheKey := getServerJoinCodeCacheKey(userID, serverID, config)

	var joinCode string
	err := cache.Get(ctx, cacheKey, &joinCode)
	if err != nil {
		if errors.Is(err, commonCache.ErrKeyNotFound) {
			logger.Error().Msg("server join code not found in cache")
			http.Error(w, ErrUnauthorizedAccess.Error(), http.StatusUnauthorized)
			return "", ErrUnauthorizedAccess
		}

		logger.Error().Err(err).Msg("could not fetch server join code from cache")
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
