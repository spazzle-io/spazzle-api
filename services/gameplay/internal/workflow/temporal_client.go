package workflow

import (
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"go.temporal.io/sdk/client"
)

type temporalClient struct {
	client client.Client
}

func NewTemporalClient(config util.Config) (Client, error) {
	opts := getTemporalClientOpts(config)
	c, err := client.Dial(opts)
	if err != nil {
		return nil, err
	}

	tc := &temporalClient{
		client: c,
	}

	return tc, nil
}

func (tc *temporalClient) Close() {
	tc.client.Close()
}
