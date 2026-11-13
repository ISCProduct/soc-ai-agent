package repository

import (
	"Backend/internal/models"
	"context"
)

// ReleaseNoteRepository は「更新情報」(release_notes)の永続化インターフェース。
type ReleaseNoteRepository interface {
	// DeleteByPRNumber は PR 番号が一致する行を削除する。
	DeleteByPRNumber(ctx context.Context, prNumber uint) error
	// ListAll は全件を返す(開発者向けの掃除用)。
	ListAll(ctx context.Context) ([]models.ReleaseNote, error)
	// Delete は1件を削除する。
	Delete(ctx context.Context, note *models.ReleaseNote) error
	// ExistsByPRNumber は PR 番号が一致する行があるかを返す。
	ExistsByPRNumber(ctx context.Context, prNumber uint) (bool, error)
	// Create は1件を保存する。
	Create(ctx context.Context, note *models.ReleaseNote) error
	// ListByAudiences は audience が audiences のいずれかの行を、新しい順に limit 件返す。
	ListByAudiences(ctx context.Context, audiences []string, limit int) ([]models.ReleaseNote, error)
}
