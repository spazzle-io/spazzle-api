package deps

import (
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonStorage "github.com/spazzle-io/spazzle-api/libs/common/storage"
	db "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/sqlc"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gamecache"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameserver"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/services"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/worker"
)

type APIServerDeps struct {
	Config          *util.Config
	Store           db.Store
	Cache           commonCache.Cache
	GameCache       *gamecache.GameCache
	Bus             eventbus.EventBus
	GfClient        gameflow.Client
	GsManager       *gameserver.Manager
	WordStore       wordstore.Store
	ObjectStore     commonStorage.ObjectStore
	TaskDistributor worker.TaskDistributor
	AuthService     services.AuthGrpcService
}
