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
	// ClaimSource は実行前にソースを確保する（next_run_at を now+lease へ先送り）。
	// onlyIfDue=true（定期実行）は next_run_at<=now のときだけ、false（手動実行）は確保中でなければ確保する。
	ClaimSource(ctx context.Context, id uint, now time.Time, lease time.Duration, onlyIfDue bool) (bool, error)
	CreateRun(run *models.CrawlRun) error
	UpdateRun(run *models.CrawlRun) error
	ListRuns(sourceID uint, limit int) ([]models.CrawlRun, error)
}
