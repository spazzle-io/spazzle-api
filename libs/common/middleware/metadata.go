package middleware

import (
	"context"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func GrpcExtractMetadata(
	ctx context.Context,
	req interface{},
	_ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		userAgents := md.Get(UserAgentHeader)
		if len(userAgents) > 0 {
			ctx = context.WithValue(ctx, UserAgent, userAgents[0])
		}

		userAgents = md.Get(GrpcGatewayUserAgentHeader)
		if len(userAgents) > 0 {
			ctx = context.WithValue(ctx, UserAgent, userAgents[0])
		}

		clientIPs := md.Get(XForwardedForHeader)
		if len(clientIPs) > 0 {
			ctx = context.WithValue(ctx, ClientIP, clientIPs[0])
		}
	}

	result, err := handler(ctx, req)

	return result, err
}

func HTTPExtractMetadata(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if userAgentHeaderVal := req.Header.Get(UserAgentHeader); userAgentHeaderVal != "" {
			req = req.WithContext(context.WithValue(req.Context(), UserAgent, userAgentHeaderVal))
		}

		if xForwardedForHeaderVal := req.Header.Get(XForwardedForHeader); xForwardedForHeaderVal != "" {
			req = req.WithContext(context.WithValue(req.Context(), ClientIP, xForwardedForHeaderVal))
		}

		handler.ServeHTTP(res, req)
	})
}
