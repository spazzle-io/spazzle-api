package workflow

import (
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/client"
)

type temporalClient struct {
	client client.Client
}

func NewTemporalClient() (Client, error) {
	c, err := client.Dial(client.Options{
		Logger: NewTemporalLogger(log.Logger),
	})
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
