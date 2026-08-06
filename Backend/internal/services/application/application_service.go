package application

import (
	"Backend/domain/entity"
	"Backend/internal/repositories"
	"fmt"
	"slices"
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

// ValidStatuses 有効な選考ステータス一覧（docs/requirements/application-status-transition.md §6）
var ValidStatuses = []string{
	"not_applied",           // 未応募
	"applied",               // 応募済み
	"document_screening",    // 書類選考中
	"document_passed",       // 書類通過
	"interview_scheduled",   // 面接予定
	"interview_in_progress", // 面接中
	"offered",               // 内定
	"accepted",              // 内定承諾（終了状態）
	"withdrawn",             // 辞退（終了状態）
	"rejected",              // 不採用（終了状態）
}

// terminalStatuses 終了状態：原則として通常更新を受け付けない
var terminalStatuses = map[string]bool{
	"accepted": true,
	"withdrawn": true,
	"rejected":  true,
}

// userAllowedTransitions ユーザーが実行できる遷移（辞退・内定承諾のみ）
var userAllowedTransitions = map[string][]string{
	"applied":               {"withdrawn"},
	"document_screening":    {"withdrawn"},
	"document_passed":       {"withdrawn"},
	"interview_scheduled":   {"withdrawn"},
	"interview_in_progress": {"withdrawn"},
	"offered":               {"accepted", "withdrawn"},
}

// adminAllowedTransitions 管理者が実行できる遷移（ユーザー遷移 + 選考進捗更新）
var adminAllowedTransitions = map[string][]string{
	"not_applied":           {"applied"},
	"applied":               {"document_screening", "withdrawn"},
	"document_screening":    {"document_passed", "rejected", "withdrawn"},
	"document_passed":       {"interview_scheduled", "withdrawn"},
	"interview_scheduled":   {"interview_in_progress", "withdrawn"},
	"interview_in_progress": {"offered", "rejected", "withdrawn"},
	"offered":               {"accepted", "withdrawn"},
}

func isValidStatus(status string) bool {
	return slices.Contains(ValidStatuses, status)
}

// CanTransition 現在のステータスから次のステータスへの遷移が許可されているか検証する
func CanTransition(current, next string, isAdmin bool) bool {
	if isAdmin {
		return slices.Contains(adminAllowedTransitions[current], next)
	}
	return slices.Contains(userAllowedTransitions[current], next)
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
// isAdmin=true の場合は管理者権限の遷移が許可される
func (s *ApplicationService) UpdateStatus(applicationID uint, userID uint, status, notes string, isAdmin bool) (*entity.UserApplicationStatus, error) {
	if !isValidStatus(status) {
		return nil, fmt.Errorf("無効なステータス: %s", status)
	}

	// 所有権確認（非管理者は自分の応募のみ更新可能）
	app, err := s.appRepo.FindByID(applicationID)
	if err != nil {
		return nil, fmt.Errorf("応募データが見つかりません: %w", err)
	}
	if !isAdmin && app.UserID != userID {
		return nil, fmt.Errorf("権限がありません")
	}

	// 終了状態チェック（管理者による訂正は別エンドポイントで対応予定）
	if terminalStatuses[app.Status] {
		return nil, fmt.Errorf("application_already_closed: ステータス %s は終了状態のため更新できません", app.Status)
	}

	// 状態遷移の検証
	if !CanTransition(app.Status, status, isAdmin) {
		return nil, fmt.Errorf("invalid_status_transition: %s から %s への遷移は許可されていません", app.Status, status)
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
