package gbizinfo

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	"Backend/domain/repository"
	"Backend/internal/models"
)

// #2002: 同期の失敗を記録できなかったときも、元の同期エラーを返しつつ、記録の失敗をログに出す。

type syncFailedCompanyRepoStub struct {
	repository.CompanyRepository
	updateErr error
	updated   *models.Company
}

func (s *syncFailedCompanyRepoStub) Update(c *models.Company) error {
	s.updated = c
	return s.updateErr
}

func captureGBizLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

func TestSyncFailed_RecordFails_ReturnsSyncErrorAndLogs(t *testing.T) {
	buf := captureGBizLog(t)
	repo := &syncFailedCompanyRepoStub{updateErr: errors.New("db down")}
	svc := &GBizInfoService{companyRepo: repo}

	result, err := svc.syncFailed(&models.Company{ID: 3}, "corporate_number is required")

	if err == nil || err.Error() != "corporate_number is required" {
		t.Fatalf("元の同期エラーを返すべき: %v", err)
	}
	if result == nil || result.Status != "failed" {
		t.Fatalf("result = %+v", result)
	}
	if got := buf.String(); !strings.Contains(got, "[GBizInfo] failed to record sync failure company_id=3 (sync error: corporate_number is required): db down") {
		t.Errorf("記録の失敗がログに出ていない: %q", got)
	}
}

func TestSyncFailed_Recorded_NoExtraLog(t *testing.T) {
	buf := captureGBizLog(t)
	repo := &syncFailedCompanyRepoStub{}
	svc := &GBizInfoService{companyRepo: repo}

	if _, err := svc.syncFailed(&models.Company{ID: 3}, "timeout"); err == nil {
		t.Fatal("同期エラーを返すべき")
	}
	if repo.updated == nil || repo.updated.GBizSyncStatus != "failed" || repo.updated.GBizSyncMessage != "timeout" {
		t.Errorf("失敗が記録されていない: %+v", repo.updated)
	}
	if buf.Len() != 0 {
		t.Errorf("通常時に余計なログが出た: %q", buf.String())
	}
}
