package workflow

import (
	"fmt"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

func getTemporalNamespace(config util.Config) string {
	return fmt.Sprintf("%s-%s", config.ServiceName, config.Environment)
}
