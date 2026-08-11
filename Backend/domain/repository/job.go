package repository

import "Backend/internal/models"

// JobCategoryRepository は職種カテゴリの永続化インターフェース。
type JobCategoryRepository interface {
	FindAll() ([]models.JobCategory, error)
	FindByID(id uint) (*models.JobCategory, error)
	FindByName(name string) ([]models.JobCategory, error)
	FindByIndustry(industryID uint) ([]models.JobCategory, error)
	GetTopCategories() ([]models.JobCategory, error)
}

// GraduateEmploymentRepository は就職実績情報の永続化インターフェース。
type GraduateEmploymentRepository interface {
	Create(entry *models.GraduateEmployment) error
	FindByID(id uint) (*models.GraduateEmployment, error)
	Update(entry *models.GraduateEmployment) error
	// schoolID が非nilの場合は個別校で絞り込む。
	List(companyID, schoolID *uint, limit int) ([]models.GraduateEmployment, error)
}
