package websocketserver

import "github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"

func getTestConfig() util.Config {
	return util.Config{
		ServiceName:    "test",
		Environment:    "development",
		AllowedOrigins: []string{"http://localhost:3000"},
	}
}
