package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

// CompanyQueryRepository は CompanyRelationQueryRepository インターフェースの実装。
type CompanyQueryRepository struct {
	db *gorm.DB
}

func NewCompanyQueryRepository(db *gorm.DB) *CompanyQueryRepository {
	return &CompanyQueryRepository{db: db}
}

// GetByCompanyID 指定企業IDに関連する企業関係を取得
func (r *CompanyQueryRepository) GetByCompanyID(companyID uint) ([]models.CompanyRelation, error) {
	var relations []models.CompanyRelation
	err := r.withRealCompanyRelationEndpoints(r.db).
		Preload("Parent").
		Preload("Child").
		Preload("From").
		Preload("To").
		Where("parent_id = ? OR child_id = ? OR from_id = ? OR to_id = ?",
			companyID, companyID, companyID, companyID).
		Where("is_active = ?", true).
		Find(&relations).Error
	return relations, err
}

// GetAll 全企業関係を取得
func (r *CompanyQueryRepository) GetAll() ([]models.CompanyRelation, error) {
	var relations []models.CompanyRelation
	err := r.withRealCompanyRelationEndpoints(r.db).
		Preload("Parent").
		Preload("Child").
		Preload("From").
		Preload("To").
		Where("is_active = ?", true).
		Find(&relations).Error
	return relations, err
}

func (r *CompanyQueryRepository) withRealCompanyRelationEndpoints(db *gorm.DB) *gorm.DB {
	demoPattern := "%.example.com%"
	demoNames := []string{
		"株式会社テックイノベーション",
		"エンタープライズシステムズ株式会社",
		"クリエイティブラボ株式会社",
	}

	return db.
		Joins("LEFT JOIN companies parent_companies ON parent_companies.id = company_relations.parent_id").
		Joins("LEFT JOIN companies child_companies ON child_companies.id = company_relations.child_id").
		Joins("LEFT JOIN companies from_companies ON from_companies.id = company_relations.from_id").
		Joins("LEFT JOIN companies to_companies ON to_companies.id = company_relations.to_id").
		Where(
			"((company_relations.parent_id IS NOT NULL AND company_relations.child_id IS NOT NULL AND "+
				"COALESCE(parent_companies.website_url, '') NOT LIKE ? AND COALESCE(child_companies.website_url, '') NOT LIKE ? AND "+
				"COALESCE(parent_companies.name, '') NOT IN ? AND COALESCE(child_companies.name, '') NOT IN ?) OR "+
				"(company_relations.from_id IS NOT NULL AND company_relations.to_id IS NOT NULL AND "+
				"COALESCE(from_companies.website_url, '') NOT LIKE ? AND COALESCE(to_companies.website_url, '') NOT LIKE ? AND "+
				"COALESCE(from_companies.name, '') NOT IN ? AND COALESCE(to_companies.name, '') NOT IN ?))",
			demoPattern, demoPattern, demoNames, demoNames, demoPattern, demoPattern, demoNames, demoNames,
		)
}

// GetMarketInfoByCompanyID 指定企業の市場情報を取得
func (r *CompanyQueryRepository) GetMarketInfoByCompanyID(companyID uint) (*models.CompanyMarketInfo, error) {
	var marketInfo models.CompanyMarketInfo
	err := r.db.
		Preload("Company").
		Where("company_id = ?", companyID).
		First(&marketInfo).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &marketInfo, nil
}

// GetAllMarketInfo 全企業の市場情報を取得
func (r *CompanyQueryRepository) GetAllMarketInfo() ([]models.CompanyMarketInfo, error) {
	var marketInfos []models.CompanyMarketInfo
	err := r.db.
		Preload("Company").
		Find(&marketInfos).Error
	return marketInfos, err
}

// GetJobPositionsByCompany 企業の公開済み求人一覧を取得
func (r *CompanyQueryRepository) GetJobPositionsByCompany(companyID uint) ([]models.CompanyJobPosition, error) {
	var positions []models.CompanyJobPosition
	err := r.db.
		Where("company_id = ? AND is_active = ? AND data_status = ?", companyID, true, "published").
		Preload("JobCategory").
		Order("created_at desc").
		Find(&positions).Error
	return positions, err
}

// GetCompanyByID 指定IDの企業を取得
func (r *CompanyQueryRepository) GetCompanyByID(id uint) (*models.Company, error) {
	var company models.Company
	err := r.db.Where("id = ? AND is_active = ?", id, true).First(&company).Error
	if err != nil {
		return nil, err
	}
	return &company, nil
}

// GetCompaniesFiltered フィルタリングされた企業一覧と総件数を取得
func (r *CompanyQueryRepository) GetCompaniesFiltered(limit, offset int, industry, name, tech string) ([]models.Company, int64, error) {
	var total int64
	if err := applyCompanyFilters(r.db.Model(&models.Company{}), industry, name, tech).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	order := "RAND()"
	if name != "" {
		order = "name ASC"
	}

	var companies []models.Company
	err := applyCompanyFilters(r.db, industry, name, tech).
		Limit(limit).
		Offset(offset).
		Order(order).
		Find(&companies).Error
	return companies, total, err
}

func applyCompanyFilters(db *gorm.DB, industry, name, tech string) *gorm.DB {
	db = db.Where("is_active = ?", true)
	if industry != "" {
		db = db.Where("industry = ?", industry)
	}
	if name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	if tech != "" {
		like := "%" + tech + "%"
		db = db.Where("tech_stack LIKE ? OR infra_stack LIKE ? OR cicd_tools LIKE ?", like, like, like)
	}
	return db
}
