package runtime

import (
	"fmt"

	"github.com/rs/zerolog/log"
	gameflowLogger "github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/logger"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"go.temporal.io/sdk/client"
)

func getTemporalNamespace(config util.Config) string {
	return fmt.Sprintf("%s-%s", config.ServiceName, config.Environment)
}

func getTemporalClientOpts(config util.Config) client.Options {
	opts := client.Options{
		Namespace: getTemporalNamespace(config),
		Logger:    gameflowLogger.NewGlobalLogger(log.Logger),
	}

	if !config.IsDevelopmentEnvironment() {
		opts.HostPort = config.TemporalHostPort
		opts.Credentials = client.NewAPIKeyStaticCredentials(config.TemporalAPIKey)
	}

	return opts
}
