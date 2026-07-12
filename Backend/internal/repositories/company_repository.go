package repositories

import (
	"Backend/internal/models"
	"strings"
	"time"

	"gorm.io/gorm"
)

type CompanyRepository struct {
	db *gorm.DB
}

func NewCompanyRepository(db *gorm.DB) *CompanyRepository {
	return &CompanyRepository{db: db}
}

func (r *CompanyRepository) DB() *gorm.DB {
	return r.db
}

// FindAllActive アクティブな企業をページネーション付きで取得
func (r *CompanyRepository) FindAllActive(limit, offset int) ([]models.Company, error) {
	var companies []models.Company
	err := r.db.Where("is_active = ?", true).
		Order("id desc").
		Limit(limit).Offset(offset).
		Find(&companies).Error
	return companies, err
}

// CountActive アクティブ企業数を取得
func (r *CompanyRepository) CountActive() (int64, error) {
	var count int64
	err := r.db.Model(&models.Company{}).
		Where("is_active = ?", true).
		Count(&count).Error
	return count, err
}

// FindAllActiveNames アクティブ企業のIDと名前を取得（フィルタ q で部分一致）
func (r *CompanyRepository) FindAllActiveNames(q string) ([]models.CompanyName, error) {
	var names []models.CompanyName
	query := r.db.Model(&models.Company{}).Select("id, name").Where("is_active = ?", true)
	if q = strings.TrimSpace(q); q != "" {
		query = query.Where("name LIKE ?", "%"+q+"%")
	}
	err := query.Order("name asc").Find(&names).Error
	return names, err
}

// FindAllPublished 公開済み企業をページネーション付きで取得（マッチング用）
func (r *CompanyRepository) FindAllPublished(limit, offset int) ([]models.Company, error) {
	var companies []models.Company
	err := r.db.Where("is_active = ? AND data_status = ?", true, "published").
		Order("id desc").
		Limit(limit).Offset(offset).
		Find(&companies).Error
	return companies, err
}

// CountPublished 公開済み企業数を取得
func (r *CompanyRepository) CountPublished() (int64, error) {
	var count int64
	err := r.db.Model(&models.Company{}).
		Where("is_active = ? AND data_status = ?", true, "published").
		Count(&count).Error
	return count, err
}

// FindByID IDで企業を取得
func (r *CompanyRepository) FindByID(id uint) (*models.Company, error) {
	var company models.Company
	err := r.db.First(&company, id).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// FindByName 企業名で取得
func (r *CompanyRepository) FindByName(name string) (*models.Company, error) {
	var company models.Company
	err := r.db.Where("name = ?", name).First(&company).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// FindByCorporateNumber 法人番号で企業を取得
func (r *CompanyRepository) FindByCorporateNumber(corporateNumber string) (*models.Company, error) {
	var company models.Company
	err := r.db.Where("corporate_number = ?", corporateNumber).First(&company).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// GetWeightProfile 企業の重視度プロファイルを取得
func (r *CompanyRepository) GetWeightProfile(companyID uint, jobPositionID *uint) (*models.CompanyWeightProfile, error) {
	var profile models.CompanyWeightProfile
	query := r.db.Where("company_id = ?", companyID)

	if jobPositionID != nil {
		query = query.Where("job_position_id = ?", *jobPositionID)
	} else {
		query = query.Where("job_position_id IS NULL")
	}

	err := query.First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// Create 企業を作成
func (r *CompanyRepository) Create(company *models.Company) error {
	return r.db.Create(company).Error
}

// Update 企業情報を更新
func (r *CompanyRepository) Update(company *models.Company) error {
	return r.db.Save(company).Error
}

// FindJobPositionByCompanyAndTitle 企業IDと職種タイトルで募集職種を取得
func (r *CompanyRepository) FindJobPositionByCompanyAndTitle(companyID uint, title string) (*models.CompanyJobPosition, error) {
	var position models.CompanyJobPosition
	err := r.db.Where("company_id = ? AND title = ?", companyID, title).First(&position).Error
	if err != nil {
		return nil, err
	}
	return &position, nil
}

// FindJobPositionByID 募集職種をIDで取得
func (r *CompanyRepository) FindJobPositionByID(id uint) (*models.CompanyJobPosition, error) {
	var position models.CompanyJobPosition
	err := r.db.First(&position, id).Error
	if err != nil {
		return nil, err
	}
	return &position, nil
}

// CreateJobPosition 募集職種を作成
func (r *CompanyRepository) CreateJobPosition(position *models.CompanyJobPosition) error {
	db := r.db
	if position.JobCategoryID == 0 {
		db = db.Omit("JobCategoryID")
	}
	return db.Create(position).Error
}

// UpdateJobPosition 募集職種を更新
func (r *CompanyRepository) UpdateJobPosition(position *models.CompanyJobPosition) error {
	return r.db.Save(position).Error
}

// FindJobPositionsByCompany 企業の公開済み募集職種を取得（公開ユーザー向け）
func (r *CompanyRepository) FindJobPositionsByCompany(companyID uint) ([]models.CompanyJobPosition, error) {
	var positions []models.CompanyJobPosition
	err := r.db.Where("company_id = ? AND is_active = ? AND data_status = ?", companyID, true, "published").
		Preload("JobCategory").
		Find(&positions).Error
	return positions, err
}

// ListJobPositions 募集職種を一覧取得
func (r *CompanyRepository) ListJobPositions(companyID *uint, limit int) ([]models.CompanyJobPosition, error) {
	if limit <= 0 {
		limit = 50
	}
	var positions []models.CompanyJobPosition
	query := r.db.Preload("JobCategory").Preload("Company")
	if companyID != nil {
		query = query.Where("company_id = ?", *companyID)
	}
	err := query.Order("created_at desc").Limit(limit).Find(&positions).Error
	return positions, err
}

// CreateOrUpdateWeightProfile 重視度プロファイルを作成または更新
func (r *CompanyRepository) CreateOrUpdateWeightProfile(profile *models.CompanyWeightProfile) error {
	var existing models.CompanyWeightProfile
	query := r.db.Where("company_id = ?", profile.CompanyID)

	if profile.JobPositionID != nil {
		query = query.Where("job_position_id = ?", *profile.JobPositionID)
	} else {
		query = query.Where("job_position_id IS NULL")
	}

	err := query.First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		// 新規作成
		return r.db.Create(profile).Error
	} else if err != nil {
		return err
	}

	// 更新（created_atはDBの値を引き継ぐ）
	profile.ID = existing.ID
	profile.CreatedAt = existing.CreatedAt
	return r.db.Save(profile).Error
}

// CountWeightProfiles 企業の重視度プロファイル件数を取得
func (r *CompanyRepository) CountWeightProfiles() (int64, error) {
	var count int64
	err := r.db.Model(&models.CompanyWeightProfile{}).Count(&count).Error
	return count, err
}

// ListPublishedL1WarmCandidates は L1 未充足の公開企業を返す。
func (r *CompanyRepository) ListPublishedL1WarmCandidates(limit int, infoTTL time.Duration) ([]models.CompanyL1WarmRow, error) {
	if limit <= 0 {
		limit = 100
	}
	cutoff := time.Now().Add(-infoTTL)
	var rows []models.CompanyL1WarmRow
	err := r.db.Raw(`
SELECT
  c.*,
  CASE WHEN p.id IS NULL THEN 0 ELSE 1 END AS has_weight_profile
FROM companies c
LEFT JOIN company_weight_profiles p
  ON p.company_id = c.id AND p.job_position_id IS NULL
WHERE c.is_active = ?
  AND c.data_status = ?
  AND (
    c.info_fetched_at IS NULL
    OR c.info_fetched_at < ?
    OR p.id IS NULL
  )
ORDER BY c.id ASC
LIMIT ?
`, true, "published", cutoff, limit).Scan(&rows).Error
	return rows, err
}

// CountL1Coverage は公開カタログの L1 充足統計を返す。
func (r *CompanyRepository) CountL1Coverage(infoTTL time.Duration) (*models.L1CoverageStats, error) {
	cutoff := time.Now().Add(-infoTTL)
	stats := &models.L1CoverageStats{}
	if err := r.db.Model(&models.Company{}).
		Where("is_active = ? AND data_status = ?", true, "published").
		Count(&stats.PublishedTotal).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.Company{}).
		Where("is_active = ? AND data_status = ? AND info_fetched_at IS NOT NULL AND info_fetched_at >= ?", true, "published", cutoff).
		Count(&stats.InfoFresh).Error; err != nil {
		return nil, err
	}
	if err := r.db.Raw(`
SELECT COUNT(DISTINCT c.id)
FROM companies c
INNER JOIN company_weight_profiles p
  ON p.company_id = c.id AND p.job_position_id IS NULL
WHERE c.is_active = ? AND c.data_status = ?
`, true, "published").Scan(&stats.HasProfile).Error; err != nil {
		return nil, err
	}
	if err := r.db.Raw(`
SELECT COUNT(*)
FROM companies c
LEFT JOIN company_weight_profiles p
  ON p.company_id = c.id AND p.job_position_id IS NULL
WHERE c.is_active = ?
  AND c.data_status = ?
  AND (
    c.info_fetched_at IS NULL
    OR c.info_fetched_at < ?
    OR p.id IS NULL
  )
`, true, "published", cutoff).Scan(&stats.NeedsWarm).Error; err != nil {
		return nil, err
	}
	return stats, nil
}
