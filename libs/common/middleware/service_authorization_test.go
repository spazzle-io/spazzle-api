package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthorizeService(t *testing.T) {
	testCases := []struct {
		name            string
		inputCtx        context.Context
		allowedServices []Service
		checkResult     func(authenticatedService Service, err error)
	}{
		{
			name:            "success",
			inputCtx:        context.WithValue(context.Background(), AuthenticatedService, "authed_service"),
			allowedServices: []Service{Service("authed_service")},
			checkResult: func(authenticatedService Service, err error) {
				require.NoError(t, err)
				require.Equal(t, Service("authed_service"), authenticatedService)
			},
		},
		{
			name:            "request not made by an authenticated service",
			inputCtx:        context.Background(),
			allowedServices: []Service{Service("authed_service")},
			checkResult: func(authenticatedService Service, err error) {
				require.Error(t, err)
				require.Empty(t, authenticatedService)
			},
		},
		{
			name:            "request not made by an allowed service",
			inputCtx:        context.WithValue(context.Background(), AuthenticatedService, "some_other_service"),
			allowedServices: []Service{Service("authed_service")},
			checkResult: func(authenticatedService Service, err error) {
				require.Error(t, err)
				require.Empty(t, authenticatedService)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := AuthorizeService(tc.inputCtx, tc.allowedServices)
			tc.checkResult(result, err)
		})
	}
}
