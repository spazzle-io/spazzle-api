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
	ErrInvalidRole                = errors.New("invalid role")
	ErrInvalidUserID              = errors.New("invalid user ID")
	ErrInvalidServerID            = errors.New("invalid server ID")
)

type Role string

const (
	Player    Role = "player"
	Moderator Role = "moderator"
	Spectator Role = "spectator"
)

type WsHandler struct {
	role           Role
	userID         uuid.UUID
	serverID       uuid.UUID
	serverJoinCode string

	Config    util.Config
	Store     db.Store
	Cache     commonCache.Cache
	GfClient  gameflow.Client
	Bus       eventbus.EventBus
	GsManager *gameserver.Manager
	WordStore wordstore.Store
}

type serveWsOptions struct {
	disablePumps bool
}

type ServeWsOption func(*serveWsOptions)

func WithoutPumps() ServeWsOption {
	return func(o *serveWsOptions) {
		o.disablePumps = true
	}
}

func (h *WsHandler) ServeWs(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	opts ...ServeWsOption,
) (*gameserver.Client, error) {
	options := &serveWsOptions{}
	for _, opt := range opts {
		opt(options)
	}

	err := h.extractRequestMetadata(ctx, w, r)
	if err != nil {
		log.Error().Err(err).Msg("failed to extract metadata from ws upgrade request")
		return nil, err
	}

	logger := log.With().Str("user_id", h.userID.String()).Str("server_id", h.serverID.String()).Logger()

	validServerJoinCode, err := fetchServerJoinCode(ctx, w, h.userID, h.serverID, h.Config, h.Cache)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch server join code")
		return nil, err
	}
	if validServerJoinCode != h.serverJoinCode {
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

	gameServer, err := h.GsManager.GetOrCreateGameServer(h.serverID, gameServerConfig)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get or create game server instance")
		return nil, ErrInternalServerError
	}

	if gameServer.IsClosed() {
		logger.Warn().Msg("game server already closed. attempting to create a new game server")
		h.GsManager.RemoveGameServerIfClosed(h.serverID)
		gameServer, err = h.GsManager.GetOrCreateGameServer(h.serverID, gameServerConfig)
		if err != nil {
			logger.Error().Err(err).Msg("failed to get or create game server instance")
			return nil, ErrInternalServerError
		}
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		EnableCompression: true,
		CheckOrigin:       h.checkOrigin(h.userID),
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error().Err(err).Msg("failed to upgrade HTTP server connection to websocket protocol")
		http.Error(w, ErrInternalServerError.Error(), http.StatusInternalServerError)
		return nil, err
	}

	isClientSpectating := h.role != Player

	var clientOpts []gameserver.ClientOption
	if options.disablePumps {
		clientOpts = append(clientOpts, gameserver.WithoutPumps())
	}

	client, err := gameserver.NewClient(ctx, gameServer, conn, h.userID, isClientSpectating, clientOpts...)
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

func (h *WsHandler) extractRequestMetadata(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	var err error

	role := r.URL.Query().Get("role")
	userIDStr := r.URL.Query().Get("user_id")
	serverIDStr := r.URL.Query().Get("server_id")

	if userIDStr == "" {
		log.Error().Msg("missing user ID in ws upgrade request")
		http.Error(w, ErrMissingUserID.Error(), http.StatusBadRequest)
		return ErrMissingUserID
	}

	logger := log.With().Str("user_id", userIDStr).Logger()

	h.userID, err = uuid.Parse(userIDStr)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse user ID from ws upgrade request")
		http.Error(w, ErrInvalidUserID.Error(), http.StatusBadRequest)
		return ErrInvalidUserID
	}

	if serverIDStr == "" {
		logger.Error().Msg("missing server ID in ws upgrade request")
		http.Error(w, ErrMissingServerID.Error(), http.StatusBadRequest)
		return ErrMissingServerID
	}

	logger = logger.With().Str("server_id", serverIDStr).Logger()

	h.serverID, err = uuid.Parse(serverIDStr)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse server ID from ws upgrade request")
		http.Error(w, ErrInvalidServerID.Error(), http.StatusBadRequest)
		return ErrInvalidServerID
	}

	h.role, err = ParseRole(role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	if h.role == Moderator {
		if err := h.authorizeModerator(ctx); err != nil {
			status := http.StatusUnauthorized
			if errors.Is(err, ErrInternalServerError) {
				status = http.StatusInternalServerError
			}
			http.Error(w, err.Error(), status)
			return err
		}
	}

	authHeader := r.Header.Get(commonMiddleware.AuthorizationHeader)
	if authHeader == "" {
		logger.Error().Msg("missing authorization header in ws upgrade request")
		http.Error(w, ErrMissingAuthorizationHeader.Error(), http.StatusUnauthorized)
		return ErrMissingAuthorizationHeader
	}

	if !strings.HasPrefix(strings.ToLower(authHeader), commonMiddleware.AuthorizationBearer) {
		logger.Error().Msg("invalid authorization header in ws upgrade request")
		http.Error(w, ErrInvalidAuthorizationHeader.Error(), http.StatusUnauthorized)
		return ErrInvalidAuthorizationHeader
	}

	h.serverJoinCode = strings.TrimSpace(authHeader[len(commonMiddleware.AuthorizationBearer):])

	return nil
}

func ParseRole(queryVal string) (Role, error) {
	role := strings.TrimSpace(strings.ToLower(queryVal))

	switch role {
	case "", string(Player):
		return Player, nil
	case string(Spectator):
		return Spectator, nil
	case string(Moderator):
		return Moderator, nil
	default:
		return "", ErrInvalidRole
	}
}

func (h *WsHandler) authorizeModerator(ctx context.Context) error {
	logger := log.With().
		Str("user_id", h.userID.String()).
		Str("server_id", h.serverID.String()).
		Logger()

	permissions, err := h.Store.GetServerUserPermissions(ctx, db.GetServerUserPermissionsParams{
		ServerID: h.serverID,
		UserID:   h.userID,
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to get server user permissions")
		return ErrInternalServerError
	}

	if !permissions.HasElevatedPermissions {
		logger.Warn().Msg("user attempted to join as moderator without elevated permissions")
		return ErrUnauthorizedAccess
	}

	return nil
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
