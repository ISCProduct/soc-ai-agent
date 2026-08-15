package shared_test

import (
	"testing"

	"Backend/internal/services/shared"
)

func TestS3BucketFromEnvPrefersAWSThenLegacy(t *testing.T) {
	t.Setenv("AWS_S3_BUCKET", "")
	t.Setenv("S3_BUCKET", "legacy-bucket")
	if got := shared.S3BucketFromEnv(); got != "legacy-bucket" {
		t.Fatalf("legacy S3_BUCKET: got %q", got)
	}

	t.Setenv("AWS_S3_BUCKET", "canonical-bucket")
	if got := shared.S3BucketFromEnv(); got != "canonical-bucket" {
		t.Fatalf("AWS_S3_BUCKET should win: got %q", got)
	}
}
