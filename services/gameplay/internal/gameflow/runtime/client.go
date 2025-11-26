package runtime

import (
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
	"go.temporal.io/sdk/client"
)

type Client interface {
	Close()
}

type defaultClient struct {
	client client.Client
}

func NewClient(config util.Config) (Client, error) {
	opts := getTemporalClientOpts(config)
	c, err := client.Dial(opts)
	if err != nil {
		return nil, err
	}

	tc := &defaultClient{
		client: c,
	}

	return tc, nil
}

func (tc *defaultClient) Close() {
	tc.client.Close()
}
