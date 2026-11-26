package repository

import (
	"context"

	"Backend/internal/models"
)

// JobCategoryRepository は職種カテゴリの永続化インターフェース。
// context はリクエストのキャンセルと締切をクエリへ渡す。
type JobCategoryRepository interface {
	FindAll(ctx context.Context) ([]models.JobCategory, error)
	FindByID(ctx context.Context, id uint) (*models.JobCategory, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.JobCategory, error)
	FindByName(ctx context.Context, name string) ([]models.JobCategory, error)
	GetTopCategories(ctx context.Context) ([]models.JobCategory, error)
}

// GraduateEmploymentRepository は就職実績情報の永続化インターフェース。
type GraduateEmploymentRepository interface {
	Create(entry *models.GraduateEmployment) error
	FindByID(id uint) (*models.GraduateEmployment, error)
	Update(entry *models.GraduateEmployment) error
	// schoolID が非nilの場合は個別校で絞り込む。
	List(companyID, schoolID *uint, limit int) ([]models.GraduateEmployment, error)
	// PlacementFacts は集計用。氏名は返さない。schoolID が非nilのときはその校だけ。
	PlacementFacts(ctx context.Context, schoolID *uint) ([]models.GraduatePlacement, error)
}
