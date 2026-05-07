package repository

import "Backend/internal/models"

// CompanyRelationQueryRepository は企業関係情報の読み取り専用インターフェース。
// CompanyRelationController で使用する。
type CompanyRelationQueryRepository interface {
	GetByCompanyID(companyID uint) ([]models.CompanyRelation, error)
	GetAll() ([]models.CompanyRelation, error)
	GetMarketInfoByCompanyID(companyID uint) (*models.CompanyMarketInfo, error)
	GetAllMarketInfo() ([]models.CompanyMarketInfo, error)
	GetJobPositionsByCompany(companyID uint) ([]models.CompanyJobPosition, error)
	GetCompaniesFiltered(limit, offset int, industry, name, tech string) ([]models.Company, int64, error)
	GetCompanyByID(id uint) (*models.Company, error)
}

// CompanyRepository は企業情報の永続化インターフェース。
type CompanyRepository interface {
	FindAllActive(limit, offset int) ([]models.Company, error)
	CountActive() (int64, error)
	FindAllPublished(limit, offset int) ([]models.Company, error)
	CountPublished() (int64, error)
	FindByID(id uint) (*models.Company, error)
	FindByName(name string) (*models.Company, error)
	FindByCorporateNumber(corporateNumber string) (*models.Company, error)
	GetWeightProfile(companyID uint, jobPositionID *uint) (*models.CompanyWeightProfile, error)
	Create(company *models.Company) error
	Update(company *models.Company) error
	FindJobPositionByCompanyAndTitle(companyID uint, title string) (*models.CompanyJobPosition, error)
	FindJobPositionByID(id uint) (*models.CompanyJobPosition, error)
	CreateJobPosition(position *models.CompanyJobPosition) error
	UpdateJobPosition(position *models.CompanyJobPosition) error
	FindJobPositionsByCompany(companyID uint) ([]models.CompanyJobPosition, error)
	ListJobPositions(companyID *uint, limit int) ([]models.CompanyJobPosition, error)
	CreateOrUpdateWeightProfile(profile *models.CompanyWeightProfile) error
	CountWeightProfiles() (int64, error)
}

// CompanyRelationRepository は企業間関係の永続化インターフェース。
type CompanyRelationRepository interface {
	UpsertBusinessRelation(fromID, toID uint, relationType, description string) error
}

// CompanyPopularityRepository は企業人気情報の永続化インターフェース。
type CompanyPopularityRepository interface {
	Create(record *models.CompanyPopularityRecord) error
	ListByCompany(companyID uint, limit int) ([]models.CompanyPopularityRecord, error)
}

// GBizInfoRepository は gBizINFO データの永続化インターフェース。
type GBizInfoRepository interface {
	UpsertProfile(profile *models.GBizCompanyProfile) error
	ReplaceProcurements(companyID uint, rows []models.GBizProcurement) error
	ReplaceSubsidies(companyID uint, rows []models.GBizSubsidy) error
	ReplaceFinances(companyID uint, rows []models.GBizFinance) error
	UpsertWorkplace(workplace *models.GBizWorkplace) error
}
