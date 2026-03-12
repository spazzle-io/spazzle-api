package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

var (
	testStore  ObjectStore
	testBucket string
	testClient *s3.Client
)

func TestMain(m *testing.M) {
	endpoint := "http://localhost:9000"
	region := "us-east-1"
	accessKey := "minioadmin"
	secretKey := "minioadmin"

	testBucket = fmt.Sprintf("test-%s", uuid.New().String())

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	testClient = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	_, err = testClient.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		log.Fatalf("failed to create test bucket: %v", err)
	}

	store, err := NewS3Store(endpoint, region, accessKey, secretKey)
	if err != nil {
		log.Fatalf("failed to create store: %v", err)
	}
	testStore = store

	code := m.Run()

	cleanupBucket(context.Background())

	os.Exit(code)
}

func cleanupBucket(ctx context.Context) {
	list, err := testClient.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		log.Fatalf("failed to list objects for cleanup: %v", err)
	}

	for _, obj := range list.Contents {
		_, err := testClient.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(testBucket),
			Key:    obj.Key,
		})
		if err != nil {
			log.Fatalf("failed to delete objects during cleanup: %v", err)
		}
	}

	_, err = testClient.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(testBucket),
	})
	if err != nil {
		log.Fatalf("failed to delete test bucket: %v", err)
	}
}
