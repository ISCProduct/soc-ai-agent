package schoolapproval

import (
	"errors"
	"testing"

	"Backend/internal/models"

	"github.com/go-sql-driver/mysql"
)

type fakeAppRepo struct {
	byID          *models.SchoolCompanyApplication
	listed        []models.SchoolCompanyApplication
	updatedID     uint
	updatedStatus string
	updatedBy     uint
	updateErr     error
}

func (f *fakeAppRepo) ListBySchool(schoolID uint, status string) ([]models.SchoolCompanyApplication, error) {
	return f.listed, nil
}
func (f *fakeAppRepo) FindByID(id uint) (*models.SchoolCompanyApplication, error) {
	return f.byID, nil
}
func (f *fakeAppRepo) UpdateStatus(id uint, status string, reviewedBy uint) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updatedID, f.updatedStatus, f.updatedBy = id, status, reviewedBy
	return nil
}

type fakeApprovalWriter struct {
	added *models.SchoolCompanyApproval
	err   error
}

func (f *fakeApprovalWriter) AddCompanyApproval(a *models.SchoolCompanyApproval) error {
	if f.err != nil {
		return f.err
	}
	f.added = a
	return nil
}

func pendingApp() *models.SchoolCompanyApplication {
	return &models.SchoolCompanyApplication{ID: 10, SchoolID: 3, CompanyID: 7, Status: models.SchoolCompanyApplicationPending}
}

func TestApprove_Success(t *testing.T) {
	apps := &fakeAppRepo{byID: pendingApp()}
	appr := &fakeApprovalWriter{}
	svc := NewReviewService(apps, appr)
	got, err := svc.Approve(10, 3, 99)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if appr.added == nil || appr.added.CompanyID != 7 || appr.added.SchoolID != 3 {
		t.Errorf("approval not written correctly: %+v", appr.added)
	}
	if apps.updatedStatus != models.SchoolCompanyApplicationApproved || apps.updatedBy != 99 {
		t.Errorf("status update wrong: %s by %d", apps.updatedStatus, apps.updatedBy)
	}
	if got.Status != models.SchoolCompanyApplicationApproved {
		t.Errorf("returned status = %s", got.Status)
	}
}

// TestApprove_Idempotent は既に承認済みリストにある企業でも approve が通ることを検証する。
func TestApprove_Idempotent(t *testing.T) {
	apps := &fakeAppRepo{byID: pendingApp()}
	appr := &fakeApprovalWriter{err: &mysql.MySQLError{Number: 1062}}
	svc := NewReviewService(apps, appr)
	if _, err := svc.Approve(10, 3, 99); err != nil {
		t.Fatalf("duplicate approval should be idempotent, got: %v", err)
	}
	if apps.updatedStatus != models.SchoolCompanyApplicationApproved {
		t.Error("status should still be updated on idempotent approve")
	}
}

// TestApprove_ApprovalFailsNoStatusUpdate は承認リスト追加が失敗したら status を変えないことを検証する。
// ここが崩れると「承認済み表示なのに学生に企業が出ない」不整合になる。
func TestApprove_ApprovalFailsNoStatusUpdate(t *testing.T) {
	apps := &fakeAppRepo{byID: pendingApp()}
	appr := &fakeApprovalWriter{err: errors.New("db down")}
	svc := NewReviewService(apps, appr)
	if _, err := svc.Approve(10, 3, 99); err == nil {
		t.Fatal("expected error")
	}
	if apps.updatedStatus != "" {
		t.Error("status must not be updated when approval write fails")
	}
}

// TestApprove_CrossSchool は他校の申請IDを承認できないことを検証する（#1157）。
func TestApprove_CrossSchool(t *testing.T) {
	apps := &fakeAppRepo{byID: &models.SchoolCompanyApplication{ID: 10, SchoolID: 999, CompanyID: 7, Status: "pending"}}
	svc := NewReviewService(apps, &fakeApprovalWriter{})
	_, err := svc.Approve(10, 3, 99)
	if !errors.Is(err, ErrApplicationNotFound) {
		t.Fatalf("err = %v, want ErrApplicationNotFound", err)
	}
}

func TestApprove_NotFound(t *testing.T) {
	svc := NewReviewService(&fakeAppRepo{byID: nil}, &fakeApprovalWriter{})
	_, err := svc.Approve(10, 3, 99)
	if !errors.Is(err, ErrApplicationNotFound) {
		t.Fatalf("err = %v, want ErrApplicationNotFound", err)
	}
}

// TestApprove_AlreadyProcessed は処理済みの再審査を弾くことを検証する。
func TestApprove_AlreadyProcessed(t *testing.T) {
	app := pendingApp()
	app.Status = models.SchoolCompanyApplicationApproved
	svc := NewReviewService(&fakeAppRepo{byID: app}, &fakeApprovalWriter{})
	_, err := svc.Approve(10, 3, 99)
	if !errors.Is(err, ErrNotPending) {
		t.Fatalf("err = %v, want ErrNotPending", err)
	}
}

func TestReject_Success(t *testing.T) {
	apps := &fakeAppRepo{byID: pendingApp()}
	appr := &fakeApprovalWriter{}
	svc := NewReviewService(apps, appr)
	if _, err := svc.Reject(10, 3, 99); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if apps.updatedStatus != models.SchoolCompanyApplicationRejected {
		t.Errorf("status = %s, want rejected", apps.updatedStatus)
	}
	if appr.added != nil {
		t.Error("reject must not touch approval list")
	}
}
