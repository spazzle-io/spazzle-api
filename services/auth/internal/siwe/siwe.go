package siwe

import (
	"context"
	_ "embed"
	"errors"
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

var (
	siweConfig      *Config
	ErrInvalidInput = errors.New("invalid input")
)

type Payload struct {
	Nonce         string
	Message       string
	WalletAddress string
	IssuedAt      time.Time
	ExpiresAt     time.Time
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
	chainId uint32,
	walletAddress string,
) (*Payload, error) {
	walletAddress = commonUtil.NormalizeHexString(walletAddress)
	if !common.IsHexAddress(walletAddress) {
		return nil, fmt.Errorf("%w: invalid wallet address: %s", ErrInvalidInput, walletAddress)
	}

	isDomainAllowed := isDomainAllowed(domain, config.AllowedOrigins)
	if !isDomainAllowed {
		return nil, fmt.Errorf("%w: domain %s is not allowed", ErrInvalidInput, domain)
	}

	chain := siweConfig.getChain(chainId, string(config.Environment))
	if chain == nil {
		return nil, fmt.Errorf("%w: chain %d is not supported", ErrInvalidInput, chainId)
	}

	parsedUri, err := url.ParseRequestURI(strings.TrimSpace(uri))
	if err != nil {
		return nil, fmt.Errorf("%w: could not parse uri %s", ErrInvalidInput, uri)
	}

	uriHostName := strings.TrimPrefix(parsedUri.Hostname(), "www.")
	if uriHostName != domain {
		return nil, fmt.Errorf("%w: uri hostname: %s does not match provided domain: %s", ErrInvalidInput, uriHostName, domain)
	}

	if parsedUri.Scheme != "https" && config.Environment != util.Development {
		return nil, fmt.Errorf("%w: uri %s is using an unsupported scheme %s", ErrInvalidInput, uri, parsedUri.Scheme)
	}

	// Remove uri parameters and fragments
	parsedUri.RawQuery = ""
	parsedUri.Fragment = ""

	nonce, err := commonUtil.GenerateRandomNumericString(nonceLength)
	if err != nil {
		return nil, fmt.Errorf("could not generate nonce: %w", err)
	}

	issuedAt := time.Now().UTC()
	expirationTime := issuedAt.UTC().Add(expiration)

	issuedAtFormatted := issuedAt.Format("2006-01-02T15:04:05Z")
	expirationTimeFormatted := expirationTime.Format("2006-01-02T15:04:05Z")

	message := fmt.Sprintf(
		template,
		domain, walletAddress, domain, parsedUri.String(), version, chainId, nonce, issuedAtFormatted, expirationTimeFormatted,
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
	walletAddress = commonUtil.NormalizeHexString(walletAddress)
	if !common.IsHexAddress(walletAddress) {
		return "", fmt.Errorf("%w: invalid wallet address: %s", ErrInvalidInput, walletAddress)
	}

	cacheKey := fmt.Sprintf("%s-%s:%s", config.ServiceName, prefix, walletAddress)

	res, err := cache.Get(ctx, cacheKey)
	if err != nil {
		return "", fmt.Errorf("could not fetch SIWE message from cache: %w", err)
	}

	if res == nil {
		return "", fmt.Errorf("SIWE message not found in cache")
	}

	message, ok := res.(string)
	if !ok {
		return "", errors.New("could not cast SIWE message to string")
	}

	err = cache.Del(ctx, cacheKey)
	if err != nil {
		return "", fmt.Errorf("could not delete SIWE message from cache: %w", err)
	}

	return message, nil
}

func isDomainAllowed(domain string, allowedOrigins []string) bool {
	for _, allowedOrigin := range allowedOrigins {
		parsedOrigin, _ := url.ParseRequestURI(strings.TrimSpace(allowedOrigin))
		originHostName := strings.TrimPrefix(parsedOrigin.Hostname(), "www.")
		if originHostName == domain {
			return true
		}
	}

	return false
}
