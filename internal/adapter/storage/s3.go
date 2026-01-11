package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/14mdzk/goscratch/internal/port"
)

// S3Storage implements port.Storage using S3-compatible storage
type S3Storage struct {
	client    *s3.Client
	bucket    string
	presigner *s3.PresignClient
}

// S3Config holds S3 connection configuration
type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
}

// NewS3Storage creates a new S3 storage instance
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	// Create custom credentials provider
	creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	// Create S3 client with custom endpoint if provided (for MinIO, etc.)
	var client *s3.Client
	if cfg.Endpoint != "" {
		client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // Required for most S3-compatible services
		})
	} else {
		client = s3.NewFromConfig(awsCfg)
	}

	return &S3Storage{
		client:    client,
		bucket:    cfg.Bucket,
		presigner: s3.NewPresignClient(client),
	}, nil
}

func (s *S3Storage) Upload(ctx context.Context, path string, data io.Reader, opts ...port.UploadOption) (string, error) {
	cfg := port.ApplyOptions(opts)

	// Read all data into memory (required for S3)
	// For large files, consider using multipart upload
	body, err := io.ReadAll(data)
	if err != nil {
		return "", fmt.Errorf("failed to read data: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(path),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(cfg.ContentType),
	}

	// Add metadata
	if len(cfg.Metadata) > 0 {
		input.Metadata = cfg.Metadata
	}

	// Set ACL for public files
	if cfg.Public {
		input.ACL = "public-read"
	}

	_, err = s.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to upload to s3: %w", err)
	}

	return path, nil
}

func (s *S3Storage) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download from s3: %w", err)
	}

	return output.Body, nil
}

func (s *S3Storage) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from s3: %w", err)
	}
	return nil
}

func (s *S3Storage) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		// Check if it's a not found error
		return false, nil // Simplified: treat all errors as not found
	}
	return true, nil
}

func (s *S3Storage) GetURL(ctx context.Context, path string, expires time.Duration) (string, error) {
	presignedReq, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned url: %w", err)
	}

	return presignedReq.URL, nil
}

func (s *S3Storage) List(ctx context.Context, prefix string) ([]port.FileInfo, error) {
	var files []port.FileInfo
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			files = append(files, port.FileInfo{
				Path:       *obj.Key,
				Size:       *obj.Size,
				ModifiedAt: *obj.LastModified,
			})
		}
	}

	return files, nil
}

func (s *S3Storage) Close() error {
	return nil // S3 client doesn't need explicit closing
}
