package store

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Store wraps the AWS S3 client.
type S3Store struct {
	client   *s3.Client
	uploader *manager.Uploader
	bucket   string
	region   string
}

// NewS3Store creates an S3 client.  S3_BUCKET must be set.
func NewS3Store(ctx context.Context) (*S3Store, error) {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET env var is required")
	}

	transport := &http.Transport{
		MaxIdleConns:        2000,
		MaxIdleConnsPerHost: 1000,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config for S3: %w", err)
	}

	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	client := s3.NewFromConfig(cfg)

	// The S3 Transfer Manager automatically uses multipart upload for large
	// files (>5 MB) and uploads parts concurrently — critical for S15.
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = 10 * 1024 * 1024 // 10 MB parts
		u.Concurrency = 10             // concurrent part uploads per file
	})

	return &S3Store{
		client:   client,
		uploader: uploader,
		bucket:   bucket,
		region:   region,
	}, nil
}

// Upload stores data at the given key and returns its public URL.
// The bucket must have a public-read bucket policy (set up via setup.sh).
func (s *S3Store) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("s3 upload %s: %w", key, err)
	}

	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
	return url, nil
}

// Delete removes the object at key.  Not finding the key is not an error.
func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}