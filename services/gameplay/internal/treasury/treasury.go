package treasury

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/spazzle-io/safekit/pkg/chain"
	nonceredis "github.com/spazzle-io/safekit/pkg/nonce/redis"
	"github.com/spazzle-io/safekit/pkg/safe"
	"github.com/spazzle-io/safekit/pkg/signer"
	"github.com/spazzle-io/safekit/pkg/version"
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"
)

const threshold = 1

type DeployResult struct {
	Address     common.Address
	TxHash      common.Hash
	BlockNumber uint64
	GasUsed     uint64
}

type Client interface {
	PredictAddress(serverID uuid.UUID, owner common.Address) (common.Address, error)
	Deploy(ctx context.Context, serverID uuid.UUID, owner common.Address) (*DeployResult, error)
	IsDeployed(ctx context.Context, serverID uuid.UUID, owner common.Address) (bool, error)
	Close()
}

type client struct {
	safe      safe.Client
	ethClient *ethclient.Client
	rdb       *redis.Client
}

func New(config *util.Config) (Client, error) {
	var (
		s       signer.Signer
		eth     *ethclient.Client
		rdb     *redis.Client
		success bool
	)

	defer func() {
		if !success {
			if s != nil {
				s.Close()
			}

			if eth != nil {
				eth.Close()
			}

			if rdb != nil {
				_ = rdb.Close()
			}
		}
	}()

	s, err := signer.NewSignerFromHex(config.TreasuryDeployerPrivateKey)
	if err != nil {
		return nil, err
	}

	eth, err = safe.Dial(config.RPCUrl)
	if err != nil {
		return nil, err
	}

	c, err := chain.Lookup(new(big.Int).SetUint64(config.Chains.Current().ID))
	if err != nil {
		return nil, err
	}

	rdbOpts, err := redis.ParseURL(config.RedisConnURL)
	if err != nil {
		return nil, err
	}

	rdb = redis.NewClient(rdbOpts)

	nm, err := nonceredis.NewNonceManager(nonceredis.Options{
		Redis: rdb,
	})
	if err != nil {
		return nil, err
	}

	safeClient, err := safe.New(safe.Options{
		Chain:        c,
		Client:       eth,
		Signer:       s,
		Version:      version.V141,
		NonceManager: nm,
	})
	if err != nil {
		return nil, err
	}

	success = true
	return &client{
		safe:      safeClient,
		ethClient: eth,
		rdb:       rdb,
	}, nil
}

func (c *client) PredictAddress(serverID uuid.UUID, owner common.Address) (common.Address, error) {
	address, err := c.safe.PredictAddress([]common.Address{owner}, threshold, serverID[:])
	if err != nil {
		return common.Address{}, err
	}

	return address, nil
}

func (c *client) Deploy(ctx context.Context, serverID uuid.UUID, owner common.Address) (*DeployResult, error) {
	result, err := c.safe.Deploy(ctx, []common.Address{owner}, threshold, serverID[:])
	if err != nil {
		return nil, err
	}

	return toDeployResult(result), nil
}

func (c *client) IsDeployed(ctx context.Context, serverID uuid.UUID, owner common.Address) (bool, error) {
	address, err := c.safe.PredictAddress([]common.Address{owner}, threshold, serverID[:])
	if err != nil {
		return false, err
	}

	deployed, err := c.safe.IsDeployed(ctx, address)
	if err != nil {
		return false, err
	}

	return deployed, nil
}

func (c *client) Close() {
	c.safe.Close()
	c.ethClient.Close()
	_ = c.rdb.Close()
}

func toDeployResult(result *safe.DeployResult) *DeployResult {
	return &DeployResult{
		Address:     result.SafeAddress,
		TxHash:      result.TxHash,
		BlockNumber: result.BlockNumber,
		GasUsed:     result.GasUsed,
	}
}
