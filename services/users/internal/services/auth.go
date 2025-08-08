package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type AuthGrpcService interface {
	Close() error
	VerifyAccessToken(context.Context, string, *pb.VerifyAccessTokenRequest) (*pb.VerifyAccessTokenResponse, error)
	Authenticate(context.Context, string, *pb.AuthenticateRequest) (*pb.AuthenticateResponse, error)
}

type AuthServiceGrpcClient struct {
	conn   *grpc.ClientConn
	client pb.AuthServiceClient
}

func NewAuthServiceGrpcClient(serverAddress string) (AuthGrpcService, error) {
	conn, err := grpc.NewClient(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("could not create client connection to auth gRPC service: %w", err)
	}

	client := pb.NewAuthServiceClient(conn)

	return &AuthServiceGrpcClient{
		conn:   conn,
		client: client,
	}, nil
}

func (c *AuthServiceGrpcClient) Close() error {
	return c.conn.Close()
}

var getServiceAuthenticationPayload = commonMiddleware.GenerateServiceAuthenticationPayload

func populateMetadataPairs(mtdt metadata.MD, keys []string, additionalPairs map[string]string) metadata.MD {
	pairs := metadata.MD{}

	for _, key := range keys {
		vals := mtdt.Get(key)
		if len(vals) == 0 {
			continue
		}

		pairs[key] = append(pairs[key], vals...)
	}

	for k, v := range additionalPairs {
		pairs[k] = append(pairs[k], v)
	}

	return pairs
}

func (c *AuthServiceGrpcClient) withMetadata(
	ctx context.Context,
	serviceName string,
) (context.Context, error) {
	mtdt, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("could not get metadata from context")
	}

	serviceAuthenticationPayload, err := getServiceAuthenticationPayload(serviceName)
	if err != nil {
		return nil, fmt.Errorf("could not generate service authentication payload: %w", err)
	}

	md := populateMetadataPairs(
		mtdt,
		[]string{
			commonMiddleware.AuthorizationHeader,
			commonMiddleware.XForwardedForHeader,
			commonMiddleware.UserAgentHeader,
		},
		map[string]string{
			commonMiddleware.XServiceAuthenticationHeader: serviceAuthenticationPayload,
		})

	return metadata.NewOutgoingContext(ctx, md), nil
}

func (c *AuthServiceGrpcClient) VerifyAccessToken(
	ctx context.Context,
	serviceName string,
	payload *pb.VerifyAccessTokenRequest,
) (*pb.VerifyAccessTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ctx, err := c.withMetadata(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("could not add metadata to verify access token request: %w", err)
	}

	return c.client.VerifyAccessToken(ctx, payload)
}

func (c *AuthServiceGrpcClient) Authenticate(
	ctx context.Context,
	serviceName string,
	payload *pb.AuthenticateRequest,
) (*pb.AuthenticateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ctx, err := c.withMetadata(ctx, serviceName)
	if err != nil {
		return nil, fmt.Errorf("could not add metadata to authenticate request: %w", err)
	}

	return c.client.Authenticate(ctx, payload)
}
