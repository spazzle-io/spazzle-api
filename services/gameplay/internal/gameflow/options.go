package gameflow

import (
	"fmt"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/logger"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	temporalclient "go.temporal.io/sdk/client"
)

func getTemporalNamespace(config *util.Config) string {
	return fmt.Sprintf("%s-%s", config.ServiceName, config.Environment)
}

func getTemporalClientOpts(config *util.Config) temporalclient.Options {
	isDevEnv := config.Is(commonConfig.Development)

	opts := temporalclient.Options{
		Namespace: getTemporalNamespace(config),
		Logger:    logger.New(log.Logger, isDevEnv),
	}

	if !isDevEnv {
		opts.HostPort = config.TemporalHostPort
		opts.Credentials = temporalclient.NewAPIKeyStaticCredentials(config.TemporalAPIKey)
	}

	return opts
}
