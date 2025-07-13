package middleware

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
)

type Service string

func AuthorizeService(ctx context.Context, allowedServices []Service) (Service, error) {
	authenticatedServiceStr, ok := ctx.Value(AuthenticatedService).(string)
	if !ok {
		errMsg := "request not made by an authenticated service"
		log.Error().Msg(errMsg)
		return "", errors.New(errMsg)
	}

	authenticatedService := Service(authenticatedServiceStr)

	logger := log.With().Str("authenticated_service", authenticatedServiceStr).Logger()

	if !isAllowedService(authenticatedService, allowedServices) {
		errMsg := "request not made by an allowed service"
		logger.Error().Msg(errMsg)
		return "", fmt.Errorf("%s: %s", errMsg, authenticatedServiceStr)
	}

	return authenticatedService, nil
}

func isAllowedService(service Service, allowedServices []Service) bool {
	for _, allowedService := range allowedServices {
		if service == allowedService {
			return true
		}
	}

	return false
}
