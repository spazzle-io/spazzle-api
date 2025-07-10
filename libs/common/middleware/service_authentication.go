package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spazzle-io/spazzle-api/libs/common/cache"
	"github.com/spazzle-io/spazzle-api/libs/common/util"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const (
	serviceAuthenticationNonceLength     = 10
	serviceAuthenticationPayloadDuration = time.Minute
	serviceAuthenticationCacheKeyPrefix  = "service-authentication"
	serviceAuthenticationPubKeysKey      = "SERVICE_%s_PUBLIC_KEYS"
	serviceAuthenticationPrivateKeysKey  = "SERVICE_%s_PRIVATE_KEYS"
)

type AuthenticateServiceConfig struct {
	Cache cache.Cache
}

var getViperStringSlice = viper.GetStringSlice

func authenticateService(ctx context.Context, c *AuthenticateServiceConfig) context.Context {
	serviceAuthenticationVal, ok := ctx.Value(ServiceAuthentication).(string)
	if !ok {
		return ctx
	}

	logger := log.With().Str("service_auth_payload", serviceAuthenticationVal).Logger()

	splitServiceAuthenticationVal := strings.Split(serviceAuthenticationVal, ".")
	if len(splitServiceAuthenticationVal) != 4 {
		logger.Warn().Msg("invalid service authentication payload")
		return ctx
	}

	serviceName := splitServiceAuthenticationVal[0]
	reqTimestampStr := splitServiceAuthenticationVal[1]
	nonce := splitServiceAuthenticationVal[2]
	signature := splitServiceAuthenticationVal[3]

	logger = log.With().Str("authenticating_service", serviceName).Logger()

	reqTimestampInt, err := strconv.ParseInt(reqTimestampStr, 10, 64)
	if err != nil {
		logger.Warn().Msg("invalid service authentication request timestamp")
		return ctx
	}

	reqTimestamp := time.Unix(0, reqTimestampInt*int64(time.Millisecond))

	// verify the service name
	if strings.TrimSpace(serviceName) == "" {
		logger.Warn().Msg("service name must be provided")
		return ctx
	}

	// verify that the request was made within an acceptable duration
	if time.Now().UTC().After(reqTimestamp.Add(serviceAuthenticationPayloadDuration)) {
		logger.Warn().Msg("expired service authentication payload")
		return ctx
	}

	// verify nonce is of the correct length
	if len(nonce) != serviceAuthenticationNonceLength {
		logger.Warn().Msg("invalid service authentication nonce")
		return ctx
	}

	// verify the payload signature
	isSignatureValid := false
	payloadVerificationMsg := fmt.Sprintf("%s.%s.%s", serviceName, reqTimestampStr, nonce)

	authenticatingServicePubKeys := getViperStringSlice(fmt.Sprintf(serviceAuthenticationPubKeysKey, serviceName))
	if len(authenticatingServicePubKeys) == 0 {
		logger.Warn().Msg("service authentication public keys not found")
		return ctx
	}

	for _, publicKeyStr := range authenticatingServicePubKeys {
		publicKey, err := util.ParsePublicKeyFromPEM(publicKeyStr)
		if err != nil {
			logger.Warn().Str("pem", publicKeyStr).Msg("could not parse public key from PEM")
			return ctx
		}

		isSignatureValid, err = util.ECDSAVerify([]byte(payloadVerificationMsg), publicKey, signature)
		if err != nil {
			logger.Warn().Err(err).Str("pem", publicKeyStr).Msg("could not verify service authentication signature")
			return ctx
		}
	}

	if !isSignatureValid {
		logger.Warn().Msg("invalid service authentication signature")
		return ctx
	}

	cacheKey := fmt.Sprintf("%s-%s:%s", serviceName, serviceAuthenticationCacheKeyPrefix, signature)
	cachedSignature, err := c.Cache.Get(ctx, cacheKey)
	if err != nil {
		logger.Error().Err(err).Msg("could not fetch service authentication cache")
		return ctx
	}
	if cachedSignature != nil {
		logger.Error().
			Interface("cached_signature", cachedSignature).
			Msg("service authentication signature already present in cache")
		return ctx
	}

	err = c.Cache.Set(ctx, cacheKey, signature, serviceAuthenticationPayloadDuration)
	if err != nil {
		logger.Error().Err(err).Msg("could not cache service authentication signature")
		return ctx
	}

	// add authenticated service name to request context
	ctx = context.WithValue(ctx, AuthenticatedService, strings.ToLower(strings.TrimSpace(serviceName)))

	return ctx
}

func (config *AuthenticateServiceConfig) AuthenticateServiceGrpc(
	ctx context.Context,
	req any, _ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	ctx = authenticateService(ctx, config)
	return handler(ctx, req)
}

func AuthenticateServiceHTTP(handler http.Handler, config *AuthenticateServiceConfig) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := authenticateService(req.Context(), config)
		req = req.WithContext(ctx)
		handler.ServeHTTP(res, req)
	})
}

func GenerateServiceAuthenticationPayload(authenticatingServiceName string) (string, error) {
	authenticatingServiceName = strings.ToLower(strings.TrimSpace(authenticatingServiceName))
	logger := log.With().Str("authenticating_service", authenticatingServiceName).Logger()

	servicePrivateKeys := getViperStringSlice(fmt.Sprintf(serviceAuthenticationPrivateKeysKey, authenticatingServiceName))
	if len(servicePrivateKeys) == 0 {
		logger.Warn().Msg("service authentication private keys not found")
		return "", errors.New("service authentication private keys not found")
	}

	nonce, err := util.GenerateRandomAlphanumericString(serviceAuthenticationNonceLength)
	if err != nil {
		logger.Error().Err(err).Msg("could not generate service authentication nonce")
		return "", err
	}

	currentUTCTimeMillis := time.Now().UTC().UnixNano() / int64(time.Millisecond)
	currentUTCTimeMillisStr := fmt.Sprintf("%d", currentUTCTimeMillis)

	privateKey, err := util.ParsePrivateKeyFromPEM(servicePrivateKeys[len(servicePrivateKeys)-1])
	if err != nil {
		logger.Error().Err(err).Msg("could not parse private key from PEM")
		return "", err
	}

	payloadSignedMsg := fmt.Sprintf("%s.%s.%s", authenticatingServiceName, currentUTCTimeMillisStr, nonce)
	signature, err := util.ECDSASign([]byte(payloadSignedMsg), privateKey)
	if err != nil {
		log.Error().Err(err).Str("payload", payloadSignedMsg).Msg("could not sign service authentication payload")
		return "", err
	}

	payload := fmt.Sprintf("%s.%s.%s.%s", authenticatingServiceName, currentUTCTimeMillisStr, nonce, signature)

	return payload, nil
}
