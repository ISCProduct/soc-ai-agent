package flywheel

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"encoding/json"
	"fmt"
	"math"
)

const defaultMinSamples = 5

// ProfileRecalculationService 企業プロファイル再計算サービス
type ProfileRecalculationService struct {
	recalcRepo  *repositories.ProfileRecalculationRepository
	companyRepo *repositories.CompanyRepository
}

func NewProfileRecalculationService(
	recalcRepo *repositories.ProfileRecalculationRepository,
	companyRepo *repositories.CompanyRepository,
) *ProfileRecalculationService {
	return &ProfileRecalculationService{
		recalcRepo:  recalcRepo,
		companyRepo: companyRepo,
	}
}

// RecalculationResult 再計算結果レポート
type RecalculationResult struct {
	CompanyID   uint    `json:"company_id"`
	CompanyName string  `json:"company_name"`
	SampleCount int     `json:"sample_count"`
	Updated     bool    `json:"updated"`
	SkipReason  string  `json:"skip_reason,omitempty"`
}

// RecalculateAll 実績が閾値以上の全企業のプロファイルを一括再計算する
func (s *ProfileRecalculationService) RecalculateAll(minSamples int) ([]*RecalculationResult, error) {
	if minSamples <= 0 {
		minSamples = defaultMinSamples
	}

	companyIDs, err := s.recalcRepo.GetAllCompanyIDsWithPassedApplicants()
	if err != nil {
		return nil, fmt.Errorf("企業ID取得エラー: %w", err)
	}

	var results []*RecalculationResult
	for _, cid := range companyIDs {
		res, err := s.recalculateSingle(cid, minSamples, "auto_batch")
		if err != nil {
			results = append(results, &RecalculationResult{
				CompanyID:  cid,
				SkipReason: err.Error(),
			})
			continue
		}
		results = append(results, res)
	}
	return results, nil
}

// RecalculateCompany 特定企業のプロファイルを再計算する
func (s *ProfileRecalculationService) RecalculateCompany(companyID uint, minSamples int) (*RecalculationResult, error) {
	if minSamples <= 0 {
		minSamples = defaultMinSamples
	}
	return s.recalculateSingle(companyID, minSamples, "admin_manual")
}

func (s *ProfileRecalculationService) recalculateSingle(companyID uint, minSamples int, trigger string) (*RecalculationResult, error) {
	// 通過実績スコアを集計
	scores, err := s.recalcRepo.GetPassedApplicantScores(companyID)
	if err != nil {
		return nil, err
	}

	result := &RecalculationResult{CompanyID: companyID, SampleCount: scores.SampleCount}

	// 閾値チェック
	if scores.SampleCount < minSamples {
		result.SkipReason = fmt.Sprintf("実績件数不足（%d件 < 閾値%d件）", scores.SampleCount, minSamples)
		return result, nil
	}

	// 企業名を取得（レポート用）
	company, err := s.companyRepo.FindByID(companyID)
	if err == nil && company != nil {
		result.CompanyName = company.Name
	}

	// 現在のプロファイルを取得
	currentProfile, err := s.companyRepo.GetWeightProfile(companyID, nil)
	if err != nil {
		return nil, fmt.Errorf("現在のプロファイル取得エラー: %w", err)
	}

	// 更新前プロファイルを保存用にコピー
	prevProfile := *currentProfile

	// 通過者の平均スコアをプロファイルに反映（0-100 に丸め）
	newProfile := *currentProfile
	newProfile.TechnicalOrientation  = clamp(scores.AvgTechnical)
	newProfile.TeamworkOrientation   = clamp(scores.AvgTeamwork)
	newProfile.LeadershipOrientation = clamp(scores.AvgLeadership)
	newProfile.CreativityOrientation = clamp(scores.AvgCreativity)
	newProfile.StabilityOrientation  = clamp(scores.AvgStability)
	newProfile.GrowthOrientation     = clamp(scores.AvgGrowth)
	newProfile.WorkLifeBalance       = clamp(scores.AvgWorkLife)
	newProfile.ChallengeSeeking      = clamp(scores.AvgChallenge)
	newProfile.DetailOrientation     = clamp(scores.AvgDetail)
	newProfile.CommunicationSkill    = clamp(scores.AvgCommunication)

	// 変更がない場合はスキップ
	if profilesEqual(&prevProfile, &newProfile) {
		result.SkipReason = "変更なし"
		return result, nil
	}

	// 履歴を保存してからプロファイルを更新
	if err := s.recalcRepo.SaveHistory(companyID, &prevProfile, &newProfile, trigger, scores.SampleCount); err != nil {
		return nil, fmt.Errorf("履歴保存エラー: %w", err)
	}

	if err := s.companyRepo.CreateOrUpdateWeightProfile(&newProfile); err != nil {
		return nil, fmt.Errorf("プロファイル更新エラー: %w", err)
	}

	result.Updated = true
	return result, nil
}

// Rollback 指定企業のプロファイルを直前バージョンに戻す
func (s *ProfileRecalculationService) Rollback(companyID uint) error {
	history, err := s.recalcRepo.GetLatestHistory(companyID)
	if err != nil {
		return fmt.Errorf("履歴取得エラー: %w", err)
	}
	if history == nil {
		return fmt.Errorf("ロールバック可能な履歴がありません")
	}

	var prev models.CompanyWeightProfile
	if err := json.Unmarshal([]byte(history.PreviousProfile), &prev); err != nil {
		return fmt.Errorf("履歴のデシリアライズエラー: %w", err)
	}

	if err := s.companyRepo.CreateOrUpdateWeightProfile(&prev); err != nil {
		return fmt.Errorf("プロファイルロールバックエラー: %w", err)
	}
	return nil
}

// GetHistory 企業のプロファイル更新履歴を取得
func (s *ProfileRecalculationService) GetHistory(companyID uint) ([]*models.CompanyProfileUpdateHistory, error) {
	return s.recalcRepo.ListHistory(companyID, 20)
}

// clamp float64 スコアを 0-100 の整数に丸める
func clamp(v float64) int {
	return int(math.Max(0, math.Min(100, math.Round(v))))
}

// profilesEqual 10カテゴリが同一か比較
func profilesEqual(a, b *models.CompanyWeightProfile) bool {
	return a.TechnicalOrientation == b.TechnicalOrientation &&
		a.TeamworkOrientation == b.TeamworkOrientation &&
		a.LeadershipOrientation == b.LeadershipOrientation &&
		a.CreativityOrientation == b.CreativityOrientation &&
		a.StabilityOrientation == b.StabilityOrientation &&
		a.GrowthOrientation == b.GrowthOrientation &&
		a.WorkLifeBalance == b.WorkLifeBalance &&
		a.ChallengeSeeking == b.ChallengeSeeking &&
		a.DetailOrientation == b.DetailOrientation &&
		a.CommunicationSkill == b.CommunicationSkill
}
