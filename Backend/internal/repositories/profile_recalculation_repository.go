package repositories

import (
	"Backend/internal/models"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ProfileRecalculationRepository 企業プロファイル再計算用リポジトリ
type ProfileRecalculationRepository struct {
	db *gorm.DB
}

func NewProfileRecalculationRepository(db *gorm.DB) *ProfileRecalculationRepository {
	return &ProfileRecalculationRepository{db: db}
}

// PassedApplicantScores 通過実績の集計結果
type PassedApplicantScores struct {
	CompanyID        uint
	SampleCount      int
	AvgTechnical     float64
	AvgTeamwork      float64
	AvgLeadership    float64
	AvgCreativity    float64
	AvgStability     float64
	AvgGrowth        float64
	AvgWorkLife      float64
	AvgChallenge     float64
	AvgDetail        float64
	AvgCommunication float64
}

// GetPassedApplicantScores 企業ごとの通過実績スコアを集計
func (r *ProfileRecalculationRepository) GetPassedApplicantScores(companyID uint) (*PassedApplicantScores, error) {
	type row struct {
		SampleCount      int
		AvgTechnical     float64
		AvgTeamwork      float64
		AvgLeadership    float64
		AvgCreativity    float64
		AvgStability     float64
		AvgGrowth        float64
		AvgWorkLife      float64
		AvgChallenge     float64
		AvgDetail        float64
		AvgCommunication float64
	}
	var result row

	err := r.db.Table("user_application_statuses uas").
		Select(`
			COUNT(*) AS sample_count,
			AVG(ucm.technical_match)     AS avg_technical,
			AVG(ucm.teamwork_match)      AS avg_teamwork,
			AVG(ucm.leadership_match)    AS avg_leadership,
			AVG(ucm.creativity_match)    AS avg_creativity,
			AVG(ucm.stability_match)     AS avg_stability,
			AVG(ucm.growth_match)        AS avg_growth,
			AVG(ucm.work_life_match)     AS avg_work_life,
			AVG(ucm.challenge_match)     AS avg_challenge,
			AVG(ucm.detail_match)        AS avg_detail,
			AVG(ucm.communication_match) AS avg_communication
		`).
		Joins("JOIN user_company_matches ucm ON ucm.id = uas.match_id").
		Where("uas.company_id = ? AND uas.status IN ?", companyID, models.FlywheelPassedStatusFilter()).
		Scan(&result).Error
	if err != nil {
		return nil, fmt.Errorf("集計クエリエラー: %w", err)
	}

	return &PassedApplicantScores{
		CompanyID:        companyID,
		SampleCount:      result.SampleCount,
		AvgTechnical:     result.AvgTechnical,
		AvgTeamwork:      result.AvgTeamwork,
		AvgLeadership:    result.AvgLeadership,
		AvgCreativity:    result.AvgCreativity,
		AvgStability:     result.AvgStability,
		AvgGrowth:        result.AvgGrowth,
		AvgWorkLife:      result.AvgWorkLife,
		AvgChallenge:     result.AvgChallenge,
		AvgDetail:        result.AvgDetail,
		AvgCommunication: result.AvgCommunication,
	}, nil
}

// GetAllCompanyIDsWithPassedApplicants 通過実績がある企業IDを全件取得
func (r *ProfileRecalculationRepository) GetAllCompanyIDsWithPassedApplicants() ([]uint, error) {
	var ids []uint
	err := r.db.Table("user_application_statuses").
		Select("DISTINCT company_id").
		Where("status IN ?", models.FlywheelPassedStatusFilter()).
		Pluck("company_id", &ids).Error
	return ids, err
}

// SaveHistory プロファイル更新前後の履歴を保存
func (r *ProfileRecalculationRepository) SaveHistory(
	companyID uint,
	prev, next *models.CompanyWeightProfile,
	trigger string,
	sampleCount int,
) error {
	prevJSON, err := json.Marshal(prev)
	if err != nil {
		return err
	}
	nextJSON, err := json.Marshal(next)
	if err != nil {
		return err
	}
	h := &models.CompanyProfileUpdateHistory{
		CompanyID:       companyID,
		PreviousProfile: string(prevJSON),
		NewProfile:      string(nextJSON),
		Trigger:         trigger,
		SampleCount:     sampleCount,
		CreatedAt:       time.Now(),
	}
	return r.db.Create(h).Error
}

// GetLatestHistory 企業の最新の更新履歴を取得（ロールバック用）
func (r *ProfileRecalculationRepository) GetLatestHistory(companyID uint) (*models.CompanyProfileUpdateHistory, error) {
	var h models.CompanyProfileUpdateHistory
	err := r.db.Where("company_id = ?", companyID).
		Order("created_at DESC").
		First(&h).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &h, err
}

// ListHistory 企業の更新履歴一覧を取得
func (r *ProfileRecalculationRepository) ListHistory(companyID uint, limit int) ([]*models.CompanyProfileUpdateHistory, error) {
	var hs []*models.CompanyProfileUpdateHistory
	err := r.db.Where("company_id = ?", companyID).
		Order("created_at DESC").
		Limit(limit).
		Find(&hs).Error
	return hs, err
}
