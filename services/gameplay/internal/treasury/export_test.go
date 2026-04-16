package treasury

import "github.com/spazzle-io/safekit/pkg/safe"

func NewTestClient(safe safe.Client) Client {
	return &client{safe: safe}
}
