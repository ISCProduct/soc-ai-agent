package admin

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"Backend/internal/models"
)

type failAuditRepo struct {
	err     error
	created *models.AuditLog
}

func (f *failAuditRepo) Create(entry *models.AuditLog) error {
	f.created = entry
	return f.err
}

func (f *failAuditRepo) List(int) ([]models.AuditLog, error) {
	return nil, nil
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestRecordLogsWriteFailure(t *testing.T) {
	buf := captureLog(t)
	repo := &failAuditRepo{err: errors.New("db down")}
	NewAuditLogService(repo).Record("admin@example.com", "company.update", "company", 7, map[string]any{"a": 1})

	got := buf.String()
	if !strings.Contains(got, "write failed") || !strings.Contains(got, "company.update") || !strings.Contains(got, "db down") {
		t.Fatalf("log = %q", got)
	}
	if strings.Contains(got, "admin@example.com") {
		t.Fatalf("actor email leaked into log: %s", got)
	}
	if repo.created == nil || repo.created.Action != "company.update" {
		t.Fatalf("create was not attempted: %+v", repo.created)
	}
}

func TestRecordLogsMetadataMarshalFailureAndStillWrites(t *testing.T) {
	buf := captureLog(t)
	repo := &failAuditRepo{}
	NewAuditLogService(repo).Record("admin@example.com", "company.publish", "company", 3, map[string]any{"ch": make(chan int)})

	got := buf.String()
	if !strings.Contains(got, "metadata marshal failed") || !strings.Contains(got, "company.publish") {
		t.Fatalf("log = %q", got)
	}
	if repo.created == nil || repo.created.Metadata != "" {
		t.Fatalf("row = %+v, want empty metadata", repo.created)
	}
}
