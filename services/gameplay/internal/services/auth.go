package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	commonConfig "github.com/spazzle-io/spazzle-api/libs/common/config"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/util"

	commonMiddleware "github.com/spazzle-io/spazzle-api/libs/common/middleware"
	pb "github.com/spazzle-io/spazzle-api/services/proto/auth/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type AuthGrpcService interface {
	Close() error
	VerifyAccessToken(context.Context, *util.Config, *pb.VerifyAccessTokenRequest) (*pb.VerifyAccessTokenResponse, error)
}

type AuthServiceGrpcClient struct {
	conn                *grpc.ClientConn
	client              pb.AuthServiceClient
	generateAuthPayload func(*commonConfig.AppConfig) (string, error)
}

func NewAuthServiceGrpcClient(serverAddress string) (AuthGrpcService, error) {
	conn, err := grpc.NewClient(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("could not create client connection to auth gRPC service: %w", err)
	}

	client := pb.NewAuthServiceClient(conn)

	return &AuthServiceGrpcClient{
		conn:                conn,
		client:              client,
		generateAuthPayload: commonMiddleware.GenerateServiceAuthenticationPayload,
	}, nil
}

func (c *AuthServiceGrpcClient) Close() error {
	return c.conn.Close()
}

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

func (c *AuthServiceGrpcClient) withMetadata(ctx context.Context, config *util.Config) (context.Context, error) {
	mtdt, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("could not get metadata from context")
	}

	serviceAuthenticationPayload, err := c.generateAuthPayload(&config.AppConfig)
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
	config *util.Config,
	payload *pb.VerifyAccessTokenRequest,
) (*pb.VerifyAccessTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ctx, err := c.withMetadata(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("could not add metadata to verify access token request: %w", err)
	}

	return c.client.VerifyAccessToken(ctx, payload)
}
