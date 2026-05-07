package repositories

import (
	"Backend/internal/models"

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
	return r.db.Create(position).Error
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

	// 更新
	profile.ID = existing.ID
	return r.db.Save(profile).Error
}

// CountWeightProfiles 企業の重視度プロファイル件数を取得
func (r *CompanyRepository) CountWeightProfiles() (int64, error) {
	var count int64
	err := r.db.Model(&models.CompanyWeightProfile{}).Count(&count).Error
	return count, err
}
