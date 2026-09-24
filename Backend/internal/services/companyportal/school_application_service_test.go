package companyportal

import (
	"errors"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/shared"
)

// fakeSchoolAppRepo は掲載申請サービスのテスト用。
type fakeSchoolAppRepo struct {
	apps       []models.SchoolCompanyApplication
	hasPending bool
	created    *models.SchoolCompanyApplication
	found      *models.SchoolCompanyApplication
	deletedID  uint
	err        error
}

func (f *fakeSchoolAppRepo) ListByCompany(companyID uint) ([]models.SchoolCompanyApplication, error) {
	return f.apps, f.err
}
func (f *fakeSchoolAppRepo) HasPending(schoolID, companyID uint) (bool, error) {
	return f.hasPending, f.err
}
func (f *fakeSchoolAppRepo) Create(app *models.SchoolCompanyApplication) error {
	if f.err != nil {
		return f.err
	}
	f.created = app
	app.ID = 100
	return nil
}
func (f *fakeSchoolAppRepo) FindByID(id uint) (*models.SchoolCompanyApplication, error) {
	return f.found, f.err
}
func (f *fakeSchoolAppRepo) Delete(id uint) error {
	f.deletedID = id
	return f.err
}

func TestApply_Success(t *testing.T) {
	repo := &fakeSchoolAppRepo{}
	svc := NewSchoolApplicationService(repo)
	app, err := svc.Apply(7, 3, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.CompanyID != 7 || app.SchoolID != 3 || app.AppliedBy != 42 {
		t.Errorf("fields = %+v", app)
	}
	if app.Status != models.SchoolCompanyApplicationPending {
		t.Errorf("status = %q, want pending", app.Status)
	}
}

// TestApply_Duplicate は承認待ちが既にあると重複を作らないことを検証する。
// これが崩れると、キャリア担当の承認キューに同じ申請が並ぶ。
func TestApply_Duplicate(t *testing.T) {
	repo := &fakeSchoolAppRepo{hasPending: true}
	svc := NewSchoolApplicationService(repo)
	_, err := svc.Apply(7, 3, 42)
	if !errors.Is(err, ErrDuplicateApplication) {
		t.Fatalf("err = %v, want ErrDuplicateApplication", err)
	}
	if repo.created != nil {
		t.Error("重複時に作成してはいけない")
	}
}

func TestApply_MissingSchool(t *testing.T) {
	svc := NewSchoolApplicationService(&fakeSchoolAppRepo{})
	_, err := svc.Apply(7, 0, 42)
	var ve *shared.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

// TestCancel_CrossCompany は他社の申請IDを消せないことを検証する（#1156）。
func TestCancel_CrossCompany(t *testing.T) {
	repo := &fakeSchoolAppRepo{found: &models.SchoolCompanyApplication{ID: 5, CompanyID: 999}}
	svc := NewSchoolApplicationService(repo)
	err := svc.Cancel(5, 7)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if repo.deletedID != 0 {
		t.Error("他社の申請を削除してはいけない")
	}
}

// TestCancel_NotFound は存在しないIDでも 403 相当で返す（存在漏洩を防ぐ）。
func TestCancel_NotFound(t *testing.T) {
	repo := &fakeSchoolAppRepo{found: nil}
	svc := NewSchoolApplicationService(repo)
	err := svc.Cancel(5, 7)
	if !errors.Is(err, shared.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestCancel_Success(t *testing.T) {
	repo := &fakeSchoolAppRepo{found: &models.SchoolCompanyApplication{ID: 5, CompanyID: 7}}
	svc := NewSchoolApplicationService(repo)
	if err := svc.Cancel(5, 7); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.deletedID != 5 {
		t.Errorf("deletedID = %d, want 5", repo.deletedID)
	}
}
