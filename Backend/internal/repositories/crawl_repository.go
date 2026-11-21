package repositories

import (
	"Backend/internal/models"
	"context"
	"time"

	"gorm.io/gorm"
)

type CrawlRepository struct {
	db *gorm.DB
}

func NewCrawlRepository(db *gorm.DB) *CrawlRepository {
	return &CrawlRepository{db: db}
}

func (r *CrawlRepository) ListSources() ([]models.CrawlSource, error) {
	var sources []models.CrawlSource
	err := r.db.Order("created_at desc").Find(&sources).Error
	return sources, err
}

func (r *CrawlRepository) GetSource(id uint) (*models.CrawlSource, error) {
	var source models.CrawlSource
	if err := r.db.First(&source, id).Error; err != nil {
		return nil, err
	}
	return &source, nil
}

func (r *CrawlRepository) CreateSource(source *models.CrawlSource) error {
	return r.db.Create(source).Error
}

func (r *CrawlRepository) UpdateSource(source *models.CrawlSource) error {
	return r.db.Save(source).Error
}

func (r *CrawlRepository) ListDueSources(now time.Time) ([]models.CrawlSource, error) {
	var sources []models.CrawlSource
	err := r.db.
		Where("is_active = ?", true).
		Where("next_run_at IS NOT NULL AND next_run_at <= ?", now).
		Order("next_run_at asc").
		Find(&sources).Error
	return sources, err
}

// ClaimSource は条件付き UPDATE でソースを確保し、next_run_at を now+lease へ先送りする。
// 定期実行（onlyIfDue）は期限到来分だけを確保する。手動実行は next_run_at が (now, now+lease] に
// あるものを他の実行体が確保中とみなし、それ以外を確保する。
// ponytail: 手動実行では、先送り時間内に予定された定期実行も「確保中」と区別できない（409 になる）。区別が要るなら claimed_until 列を足す。
func (r *CrawlRepository) ClaimSource(ctx context.Context, id uint, now time.Time, lease time.Duration, onlyIfDue bool) (bool, error) {
	until := now.Add(lease)
	q := r.db.WithContext(ctx).Model(&models.CrawlSource{})
	if onlyIfDue {
		q = q.Where("id = ? AND next_run_at <= ?", id, now)
	} else {
		q = q.Where("id = ? AND (next_run_at IS NULL OR next_run_at <= ? OR next_run_at > ?)", id, now, until)
	}
	res := q.Update("next_run_at", until)
	return res.RowsAffected == 1, res.Error
}

func (r *CrawlRepository) CreateRun(run *models.CrawlRun) error {
	return r.db.Create(run).Error
}

func (r *CrawlRepository) UpdateRun(run *models.CrawlRun) error {
	return r.db.Save(run).Error
}

func (r *CrawlRepository) ListRuns(sourceID uint, limit int) ([]models.CrawlRun, error) {
	var runs []models.CrawlRun
	query := r.db.Order("started_at desc")
	if sourceID > 0 {
		query = query.Where("source_id = ?", sourceID)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&runs).Error
	return runs, err
}
