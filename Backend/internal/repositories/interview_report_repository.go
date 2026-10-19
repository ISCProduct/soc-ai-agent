package repositories

import (
	"Backend/internal/models"
	"time"

	"gorm.io/gorm"
)

type InterviewReportRepository struct {
	db *gorm.DB
}

func NewInterviewReportRepository(db *gorm.DB) *InterviewReportRepository {
	return &InterviewReportRepository{db: db}
}

func (r *InterviewReportRepository) FindBySessionID(sessionID uint) (*models.InterviewReport, error) {
	var report models.InterviewReport
	if err := r.db.First(&report, "session_id = ?", sessionID).Error; err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *InterviewReportRepository) Upsert(report *models.InterviewReport) error {
	return r.db.Save(report).Error
}

// ClaimScoresApplication はスコア反映の権利を1つのプロセスだけに与える（#1512）。
//
// scores_applied_at が NULL の行だけを更新するので、複数タスクが同時に呼んでも
// 1 を返すのは1つだけ。残りは 0 を受け取り、スコア反映を飛ばす。
//
// レポート生成のフォールバック経路（Redis 障害時の in-process channel）は
// プロセス内の map でしか重複排除しておらず、本番の backend は最大2タスクへ
// スケールする。排他が無いと user_weight_scores の移動平均へ二重に反映される。
//
// 反映に失敗したときは ReleaseScoresApplication で NULL へ戻すこと。
// 戻さないと asynq のリトライがスコアを書けないまま成功扱いになる。
func (r *InterviewReportRepository) ClaimScoresApplication(sessionID uint) (bool, error) {
	res := r.db.Model(&models.InterviewReport{}).
		Where("session_id = ? AND scores_applied_at IS NULL", sessionID).
		Update("scores_applied_at", time.Now())
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// ReleaseScoresApplication は確保したスコア反映の権利を返す（#1512）。
// 反映が途中で失敗したときに呼ぶ。次の試行が改めて確保できるようにする。
func (r *InterviewReportRepository) ReleaseScoresApplication(sessionID uint) error {
	return r.db.Model(&models.InterviewReport{}).
		Where("session_id = ?", sessionID).
		Update("scores_applied_at", nil).Error
}

// FindBySessionIDs は複数セッションのレポートを一括取得する
func (r *InterviewReportRepository) FindBySessionIDs(sessionIDs []uint) ([]models.InterviewReport, error) {
	if len(sessionIDs) == 0 {
		return nil, nil
	}
	var reports []models.InterviewReport
	if err := r.db.Where("session_id IN ?", sessionIDs).Find(&reports).Error; err != nil {
		return nil, err
	}
	return reports, nil
}
