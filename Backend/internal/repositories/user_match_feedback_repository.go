package repositories

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"Backend/domain/repository"
	"Backend/internal/models"
)

// UserMatchFeedbackRepository は、推薦へのフィードバックの永続化（SOCAIAGENT-670）。
type UserMatchFeedbackRepository struct {
	db *gorm.DB
}

func NewUserMatchFeedbackRepository(db *gorm.DB) *UserMatchFeedbackRepository {
	return &UserMatchFeedbackRepository{db: db}
}

// FindMatch は、判定を付ける推薦（所有者の確認と、押した時点の写しに使う）。無ければ repository.ErrNotFound。
func (r *UserMatchFeedbackRepository) FindMatch(matchID uint) (*models.UserCompanyMatch, error) {
	var m models.UserCompanyMatch
	if err := r.db.First(&m, matchID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

// Upsert は、本人×推薦の判定を保存する（押し直したら上書き）。
func (r *UserMatchFeedbackRepository) Upsert(f *models.UserMatchFeedback) error {
	return r.db.Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{
		"verdict", "match_score", "fit_mismatch_axes", "job_matched", "updated_at",
	})}).Create(f).Error
}

// Delete は、本人×推薦の判定を取り消す（無ければ何もしない）。
func (r *UserMatchFeedbackRepository) Delete(userID, matchID uint) error {
	return r.db.Where("user_id = ? AND match_id = ?", userID, matchID).Delete(&models.UserMatchFeedback{}).Error
}

// VerdictsByMatchIDs は、本人の判定を、推薦 ID から引く。
func (r *UserMatchFeedbackRepository) VerdictsByMatchIDs(userID uint, matchIDs []uint) (map[uint]string, error) {
	out := map[uint]string{}
	if len(matchIDs) == 0 {
		return out, nil
	}
	var rows []models.UserMatchFeedback
	if err := r.db.Where("user_id = ? AND match_id IN ?", userID, matchIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.MatchID] = row.Verdict
	}
	return out, nil
}
