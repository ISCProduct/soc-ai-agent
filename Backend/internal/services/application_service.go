package services

import (
	"Backend/domain/entity"
	"Backend/internal/repositories"
	"fmt"
	"time"
)

// ApplicationService 応募・選考ステータス管理サービス
type ApplicationService struct {
	appRepo   *repositories.UserApplicationStatusRepository
	matchRepo *repositories.UserCompanyMatchRepository
}

func NewApplicationService(
	appRepo *repositories.UserApplicationStatusRepository,
	matchRepo *repositories.UserCompanyMatchRepository,
) *ApplicationService {
	return &ApplicationService{appRepo: appRepo, matchRepo: matchRepo}
}

// ValidStatuses 有効な選考ステータス一覧
var ValidStatuses = []string{
	"applied",          // 応募済み
	"document_passed",  // 書類通過
	"interview",        // 面接中
	"offered",          // 内定
	"accepted",         // 内定承諾
	"declined",         // 辞退
	"rejected",         // 不合格
}

func isValidStatus(status string) bool {
	for _, s := range ValidStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// Apply 企業への応募を登録する
func (s *ApplicationService) Apply(userID, companyID, matchID uint) (*entity.UserApplicationStatus, error) {
	// 重複チェック
	existing, err := s.appRepo.FindByUserAndCompany(userID, companyID)
	if err != nil {
		return nil, fmt.Errorf("重複チェックエラー: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("この企業にはすでに応募済みです")
	}

	now := time.Now()
	app := &entity.UserApplicationStatus{
		UserID:    userID,
		CompanyID: companyID,
		MatchID:   matchID,
		Status:    "applied",
		AppliedAt: &now,
	}
	if err := s.appRepo.Create(app); err != nil {
		return nil, fmt.Errorf("応募登録エラー: %w", err)
	}

	// UserCompanyMatch の IsApplied フラグも更新
	_ = s.matchRepo.MarkAsApplied(matchID)

	return app, nil
}

// UpdateStatus 選考ステータスを更新する
func (s *ApplicationService) UpdateStatus(applicationID uint, userID uint, status, notes string) (*entity.UserApplicationStatus, error) {
	if !isValidStatus(status) {
		return nil, fmt.Errorf("無効なステータス: %s", status)
	}

	// 所有権確認
	app, err := s.appRepo.FindByID(applicationID)
	if err != nil {
		return nil, fmt.Errorf("応募データが見つかりません: %w", err)
	}
	if app.UserID != userID {
		return nil, fmt.Errorf("権限がありません")
	}

	if err := s.appRepo.UpdateStatus(applicationID, status, notes); err != nil {
		return nil, fmt.Errorf("ステータス更新エラー: %w", err)
	}

	app.Status = status
	app.Notes = notes
	return app, nil
}

// GetApplicationsByUser ユーザーの応募一覧を取得する
func (s *ApplicationService) GetApplicationsByUser(userID uint) ([]*entity.UserApplicationStatus, error) {
	return s.appRepo.FindByUserID(userID)
}

// GetCorrelation マッチングスコアと選考通過率の相関データを取得する
func (s *ApplicationService) GetCorrelation(companyID uint) ([]map[string]any, error) {
	if companyID > 0 {
		return s.appRepo.GetCorrelationByCompany(companyID)
	}
	return s.appRepo.GetGlobalCorrelation()
}
