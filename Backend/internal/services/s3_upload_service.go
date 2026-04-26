package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3UploadService uploads files to AWS S3.
type S3UploadService struct {
	bucket   string
	region   string
	client   *s3.Client
	uploader *transfermanager.Client
}

// NewS3UploadService initialises the service from environment variables
// (AWS_REGION, AWS_S3_BUCKET, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY).
func NewS3UploadService() (*S3UploadService, error) {
	bucket := os.Getenv("AWS_S3_BUCKET")
	region := os.Getenv("AWS_REGION")
	if bucket == "" {
		return nil, fmt.Errorf("AWS_S3_BUCKET is not set")
	}
	if region == "" {
		region = "ap-northeast-1"
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	uploader := transfermanager.New(client)

	return &S3UploadService{
		bucket:   bucket,
		region:   region,
		client:   client,
		uploader: uploader,
	}, nil
}

// UploadFile uploads data to S3 and returns (s3Key, publicURL, error).
func (s *S3UploadService) UploadFile(ctx context.Context, key, mimeType string, data []byte) (string, string, error) {
	return s.UploadReader(ctx, key, mimeType, bytes.NewReader(data))
}

// UploadReader streams an io.Reader to S3 using multipart upload.
// This avoids loading the entire file into memory and supports files larger than 32 MB.
func (s *S3UploadService) UploadReader(ctx context.Context, key, mimeType string, r io.Reader) (string, string, error) {
	_, err := s.uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String(mimeType),
	})
	if err != nil {
		return "", "", fmt.Errorf("s3 upload: %w", err)
	}

	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
	return key, url, nil
}

// PresignGetURL returns a presigned GET URL valid for the given duration.
// key is the S3 object key (DriveFileID stored in InterviewVideo).
func (s *S3UploadService) PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)
	req, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String("inline"),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", fmt.Errorf("presign: %w", err)
	}
	return req.URL, nil
}
