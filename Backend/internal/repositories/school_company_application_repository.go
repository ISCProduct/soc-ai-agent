package repositories

import (
	"errors"

	"Backend/internal/models"

	"gorm.io/gorm"
)

// SchoolCompanyApplicationRepository は企業→学校の掲載申請を扱う（#1506）。
type SchoolCompanyApplicationRepository struct {
	db *gorm.DB
}

func NewSchoolCompanyApplicationRepository(db *gorm.DB) *SchoolCompanyApplicationRepository {
	return &SchoolCompanyApplicationRepository{db: db}
}

// ListByCompany は企業の申請一覧を新しい順で返す。
func (r *SchoolCompanyApplicationRepository) ListByCompany(companyID uint) ([]models.SchoolCompanyApplication, error) {
	var apps []models.SchoolCompanyApplication
	err := r.db.Where("company_id = ?", companyID).
		Order("created_at DESC, id DESC").
		Find(&apps).Error
	if err != nil {
		return nil, err
	}
	return apps, nil
}

// HasPending は同一 school×company で承認待ちが既にあるか。
//
// MySQL では「pending だけ一意」を部分インデックスで表現できないため、
// アプリ側でも重複を防ぐ。一意インデックスは全 status にかけられないので、
// この関数が重複防止の主線になる。
func (r *SchoolCompanyApplicationRepository) HasPending(schoolID, companyID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.SchoolCompanyApplication{}).
		Where("school_id = ? AND company_id = ? AND status = ?",
			schoolID, companyID, models.SchoolCompanyApplicationPending).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create は申請を作る。
func (r *SchoolCompanyApplicationRepository) Create(app *models.SchoolCompanyApplication) error {
	return r.db.Create(app).Error
}

// FindByID は申請1件を返す。
func (r *SchoolCompanyApplicationRepository) FindByID(id uint) (*models.SchoolCompanyApplication, error) {
	var app models.SchoolCompanyApplication
	err := r.db.Where("id = ?", id).First(&app).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &app, nil
}

// Delete は申請を削除する。取消用。
func (r *SchoolCompanyApplicationRepository) Delete(id uint) error {
	return r.db.Delete(&models.SchoolCompanyApplication{}, id).Error
}

// ListBySchool は学校向けの申請一覧を新しい順で返す。
// status が空文字なら全件、指定があればその状態だけ。
func (r *SchoolCompanyApplicationRepository) ListBySchool(schoolID uint, status string) ([]models.SchoolCompanyApplication, error) {
	q := r.db.Where("school_id = ?", schoolID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var apps []models.SchoolCompanyApplication
	if err := q.Order("created_at DESC, id DESC").Find(&apps).Error; err != nil {
		return nil, err
	}
	return apps, nil
}

// UpdateStatus は申請の状態と審査者を更新する。承認・却下で使う。
func (r *SchoolCompanyApplicationRepository) UpdateStatus(id uint, status string, reviewedBy uint) error {
	return r.db.Model(&models.SchoolCompanyApplication{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "reviewed_by": reviewedBy}).Error
}
