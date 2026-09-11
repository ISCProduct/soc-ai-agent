package logger

import (
	"context"
	"strings"
	"testing"
	"time"
)

type stubPutter struct {
	called bool
	key    string
}

func (s *stubPutter) UploadFile(_ context.Context, key, _ string, _ []byte) (string, string, error) {
	s.called = true
	s.key = key
	return key, "", nil
}

func TestErrorLogKey(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	now := time.Date(2026, 8, 22, 8, 44, 0, 123, time.UTC)
	key := errorLogKey("fetch_missing_batch", now)
	if !strings.HasPrefix(key, "error-logs/staging/2026/08/22/fetch_missing_batch-") {
		t.Fatalf("key=%s", key)
	}
	if !strings.HasSuffix(key, ".json") {
		t.Fatalf("key=%s", key)
	}
}

func TestPutErrorJSON_SkippedOutsideStgProd(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	stub := &stubPutter{}
	EnableS3ErrorArchive(stub)
	t.Cleanup(func() { EnableS3ErrorArchive(nil) })
	if got := PutErrorJSON("fetch_missing_batch", map[string]any{"e": 1}); got != "" {
		t.Fatalf("key=%s", got)
	}
	if stub.called {
		t.Fatal("local env must not upload")
	}
}

func TestPutErrorJSON_StagingReturnsKey(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	stub := &stubPutter{}
	EnableS3ErrorArchive(stub)
	t.Cleanup(func() { EnableS3ErrorArchive(nil) })
	key := PutErrorJSON("fetch_missing_batch", map[string]any{"errors": 6})
	if !strings.HasPrefix(key, "error-logs/staging/") {
		t.Fatalf("key=%s", key)
	}
}

func TestSanitizeKind(t *testing.T) {
	if got := sanitizeKind("fetch missing/batch"); got != "fetch_missing_batch" {
		t.Fatalf("got %q", got)
	}
}

func TestArchiveEnv(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	if archiveEnv() != "production" {
		t.Fatal(archiveEnv())
	}
	t.Setenv("APP_ENV", "stg")
	if archiveEnv() != "staging" {
		t.Fatal(archiveEnv())
	}
	t.Setenv("APP_ENV", "development")
	t.Setenv("AWS_S3_BUCKET", "any")
	if archiveEnv() != "" {
		t.Fatal(archiveEnv())
	}
	t.Setenv("APP_ENV", "")
	t.Setenv("AWS_S3_BUCKET", "stg-bucket")
	if archiveEnv() != "staging" {
		t.Fatal(archiveEnv())
	}
}
