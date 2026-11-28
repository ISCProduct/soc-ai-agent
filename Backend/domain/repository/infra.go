package repository

import (
	"Backend/internal/models"
	"context"
	"time"
)

// AuditLogRepository は監査ログの永続化インターフェース。
type AuditLogRepository interface {
	Create(log *models.AuditLog) error
	List(limit int) ([]models.AuditLog, error)
}

// CrawlRepository はクロールソース・実行記録の永続化インターフェース。
type CrawlRepository interface {
	ListSources() ([]models.CrawlSource, error)
	GetSource(id uint) (*models.CrawlSource, error)
	CreateSource(source *models.CrawlSource) error
	UpdateSource(source *models.CrawlSource) error
	ListDueSources(now time.Time) ([]models.CrawlSource, error)
	CreateRun(run *models.CrawlRun) error
	UpdateRun(run *models.CrawlRun) error
	ListRuns(sourceID uint, limit int) ([]models.CrawlRun, error)
	// WithRunLock は定期取得を全タスクで1本に絞って fn を実行する。取れなければ fn を呼ばず false。
	WithRunLock(ctx context.Context, fn func()) (bool, error)
}
