package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Storage struct {
	client      *s3.Client
	tmClient    *transfermanager.Client
	bucket      string
	prefix      string
}

func newS3StorageFromEnv(ctx context.Context) (*s3Storage, error) {
	bucket := strings.TrimSpace(os.Getenv("AWS_S3_BUCKET"))
	if bucket == "" {
		return nil, nil
	}
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		return nil, errors.New("AWS_REGION is not set")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	prefix := strings.Trim(strings.TrimSpace(os.Getenv("AWS_S3_PREFIX")), "/")
	return &s3Storage{
		client:   client,
		tmClient: transfermanager.New(client),
		bucket:   bucket,
		prefix:   prefix,
	}, nil
}

func (s *s3Storage) isEnabled() bool {
	return s != nil && s.client != nil && s.bucket != ""
}

func (s *s3Storage) objectKey(parts ...string) string {
	joined := path.Join(parts...)
	if s.prefix == "" {
		return joined
	}
	return path.Join(s.prefix, joined)
}

func (s *s3Storage) uploadFile(ctx context.Context, key, filePath, contentType string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = s.tmClient.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket:      &s.bucket,
		Key:         &key,
		Body:        f,
		ContentType: &contentType,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("s3://%s/%s", s.bucket, key), nil
}

func (s *s3Storage) downloadToFile(ctx context.Context, key, destPath string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = s.tmClient.DownloadObject(ctx, &transfermanager.DownloadObjectInput{
		Bucket:   &s.bucket,
		Key:      &key,
		WriterAt: f,
	})
	return err
}

func (s *s3Storage) getObject(ctx context.Context, key string) (*s3.GetObjectOutput, error) {
	return s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
}

func parseS3URI(uri string) (string, string, bool) {
	if !strings.HasPrefix(uri, "s3://") {
		return "", "", false
	}
	trimmed := strings.TrimPrefix(uri, "s3://")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func contentTypeForPath(path string) string {
	ext := strings.ToLower(path)
	if strings.HasSuffix(ext, ".pdf") {
		return "application/pdf"
	}
	if strings.HasSuffix(ext, ".json") {
		return "application/json"
	}
	if strings.HasSuffix(ext, ".docx") {
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	return "application/octet-stream"
}

func copyToWriter(src io.Reader, dst io.Writer) error {
	_, err := io.Copy(dst, src)
	return err
}
