package siwe

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
	commonCache "github.com/spazzle-io/spazzle-api/libs/common/cache"
	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/spazzle-io/spazzle-api/services/auth/internal/util"
)

const (
	prefix      = "siwe-message"
	version     = 1
	nonceLength = 8
	expiration  = 15 * time.Minute
)

// template for the Sign In With Ethereum (SIWE) authentication message.
// See EIP-4361: https://eips.ethereum.org/EIPS/eip-4361
const template = `%s wants you to sign in with your Ethereum account:
%s

I accept the %s Terms of Service

URI: %s
Version: %d
Chain ID: %d
Nonce: %s
Issued At: %s
Expiration Time: %s`

var siweConfig *Config

type Payload struct {
	Nonce         string
	Message       string
	IssuedAt      string
	ExpiresAt     string
	WalletAddress string
}

func init() {
	var err error
	siweConfig, err = loadDefaultSIWEConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("could not load SIWE config")
	}
}

func GenerateSIWEPayload(
	ctx context.Context,
	config util.Config,
	cache commonCache.Cache,
	domain string,
	uri string,
	chainId int32,
	walletAddress string,
) (*Payload, error) {
	if !common.IsHexAddress(walletAddress) {
		return nil, fmt.Errorf("invalid wallet address: %s", walletAddress)
	}

	isDomainAllowed := siweConfig.isDomainAllowed(domain)
	if !isDomainAllowed {
		return nil, fmt.Errorf("domain %s is not allowed", domain)
	}

	chain := siweConfig.getChain(chainId, string(config.Environment))
	if chain == nil {
		return nil, fmt.Errorf("chain %d is not supported", chainId)
	}

	parsedUri, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, fmt.Errorf("could not parse uri %s", uri)
	}

	uriHostName := strings.TrimPrefix(parsedUri.Hostname(), "www.")
	if uriHostName != domain {
		return nil, fmt.Errorf("uri: %s hostname: %s does not match provided domain: %s", uri, uriHostName, domain)
	}

	if parsedUri.Scheme != "https" {
		return nil, fmt.Errorf("uri %s is using an unsupported scheme %s", uri, parsedUri.Scheme)
	}

	// Remove uri parameters and fragments
	parsedUri.RawQuery = ""
	parsedUri.Fragment = ""

	nonce, err := commonUtil.GenerateRandomNumericString(nonceLength)
	if err != nil {
		return nil, fmt.Errorf("could not generate nonce: %w", err)
	}

	issuedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	expirationTime := time.Now().UTC().Add(expiration).Format("2006-01-02T15:04:05Z")

	message := fmt.Sprintf(
		template, domain, walletAddress, domain, parsedUri.String(), version, chainId, nonce, issuedAt, expirationTime,
	)

	cacheKey := fmt.Sprintf("%s-%s:%s", config.ServiceName, prefix, walletAddress)
	err = cache.Set(ctx, cacheKey, message, expiration)
	if err != nil {
		return nil, fmt.Errorf("could not cache SIWE payload: %w", err)
	}

	payload := &Payload{
		Nonce:         nonce,
		Message:       message,
		IssuedAt:      issuedAt,
		ExpiresAt:     expirationTime,
		WalletAddress: walletAddress,
	}

	return payload, nil
}

func FetchSIWEMessage(
	ctx context.Context,
	config util.Config,
	cache commonCache.Cache,
	walletAddress string,
) (string, error) {
	cacheKey := fmt.Sprintf("%s-%s:%s", config.ServiceName, prefix, walletAddress)

	res, err := cache.Get(ctx, cacheKey)
	if err != nil {
		return "", fmt.Errorf("could not fetch SIWE message from cache: %w", err)
	}

	message, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("could not cast SIWE message to string")
	}

	err = cache.Del(ctx, cacheKey)
	if err != nil {
		return "", fmt.Errorf("could not delete SIWE message from cache: %w", err)
	}

	return message, nil
}
