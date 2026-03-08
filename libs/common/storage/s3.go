package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rs/zerolog/log"
)

type S3Store struct {
	client *s3.Client
}

func NewS3Store(endpoint string, region string, accessKey string, secretKey string) (ObjectStore, error) {
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load s3 config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	log.Info().Str("endpoint", endpoint).Msg("created object store")

	return &S3Store{client: client}, nil
}

func (s *S3Store) Put(ctx context.Context, bucket string, key string, data io.Reader, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   data,
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	if _, err := s.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("failed to put object %s/%s: %w", bucket, key, err)
	}

	return nil
}

func (s *S3Store) Get(ctx context.Context, bucket string, key string) (io.ReadCloser, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("failed to get object %s/%s: %w", bucket, key, err)
	}

	return output.Body, nil
}

func (s *S3Store) Delete(ctx context.Context, bucket string, key string) error {
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("failed to delete object %s/%s: %w", bucket, key, err)
	}

	return nil
}

func (s *S3Store) Close() error {
	return nil
}
