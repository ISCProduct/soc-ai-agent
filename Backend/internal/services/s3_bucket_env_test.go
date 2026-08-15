package services

import "testing"

func TestS3BucketFromEnvPrefersAWSThenLegacy(t *testing.T) {
	t.Setenv("AWS_S3_BUCKET", "")
	t.Setenv("S3_BUCKET", "legacy-bucket")
	if got := s3BucketFromEnv(); got != "legacy-bucket" {
		t.Fatalf("legacy S3_BUCKET: got %q", got)
	}

	t.Setenv("AWS_S3_BUCKET", "canonical-bucket")
	if got := s3BucketFromEnv(); got != "canonical-bucket" {
		t.Fatalf("AWS_S3_BUCKET should win: got %q", got)
	}
}
