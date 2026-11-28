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

// crawlRunLockName は定期取得の実行権の名前。DB 全体で1つ。
const crawlRunLockName = "crawl_run_due_sources"

// WithRunLock は定期取得を全タスクで1本に絞って fn を実行する（SOCAIAGENT-723）。
//
// 定期取得はバックエンドのプロセス内タイマーで動くため、タスクが2つ起動すると
// 同じソースを二重に実行し、外部APIの呼び出しと費用が倍になる。プロセス内の mutex
// では防げないので MySQL の GET_LOCK で排他する。待たずに諦め、次の周期に回す。
// 解放に失敗しても接続が閉じれば外れる。
func (r *CrawlRepository) WithRunLock(ctx context.Context, fn func()) (bool, error) {
	acquired := false
	err := r.db.WithContext(ctx).Connection(func(tx *gorm.DB) error {
		var got *int
		if err := tx.Raw("SELECT GET_LOCK(?, 0)", crawlRunLockName).Scan(&got).Error; err != nil {
			return err
		}
		if got == nil || *got != 1 {
			return nil
		}
		acquired = true
		defer tx.Exec("SELECT RELEASE_LOCK(?)", crawlRunLockName)
		fn()
		return nil
	})
	return acquired, err
}
