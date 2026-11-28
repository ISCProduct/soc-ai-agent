package repositories

import (
	"context"
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ResearchChatQuotaRepository は企業研究AIチャットの1日の質問回数を数える（SOCAIAGENT-724）。
type ResearchChatQuotaRepository struct {
	db *gorm.DB
}

func NewResearchChatQuotaRepository(db *gorm.DB) *ResearchChatQuotaRepository {
	return &ResearchChatQuotaRepository{db: db}
}

// Reserve は利用者の day の質問回数を1つ増やす。既に limit 回使っていれば増やさずに false を返す。
//
// 行ロック（SELECT ... FOR UPDATE）で確認と加算を1つのトランザクションにまとめるので、
// 同じ利用者が同時に質問しても limit を超えない。
func (r *ResearchChatQuotaRepository) Reserve(ctx context.Context, userID uint, day time.Time, limit int) (bool, error) {
	reserved := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := models.CompanyResearchChatQuota{UserID: userID, QuotaDate: day}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		var cur models.CompanyResearchChatQuota
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND quota_date = ?", userID, day.Format("2006-01-02")).
			Take(&cur).Error; err != nil {
			return err
		}
		if int(cur.Used) >= limit {
			return nil
		}
		reserved = true
		return tx.Model(&models.CompanyResearchChatQuota{}).
			Where("user_id = ? AND quota_date = ?", userID, day.Format("2006-01-02")).
			Updates(map[string]any{"used": gorm.Expr("used + 1"), "updated_at": time.Now()}).Error
	})
	return reserved, err
}
