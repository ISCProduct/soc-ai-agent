package application

import (
	"Backend/domain/entity"
	"Backend/domain/mapper"
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/shared"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// ApplicationService 応募・選考ステータス管理サービス
type ApplicationService struct {
	appRepo   *repositories.UserApplicationStatusRepository
	matchRepo repository.UserCompanyMatchRepository
	db        *gorm.DB
}

func NewApplicationService(
	appRepo *repositories.UserApplicationStatusRepository,
	matchRepo repository.UserCompanyMatchRepository,
	db *gorm.DB,
) *ApplicationService {
	return &ApplicationService{appRepo: appRepo, matchRepo: matchRepo, db: db}
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

// terminalStatusList 終了状態一覧（順序固定、DBクエリの IN 句にも使う）
var terminalStatusList = []string{"accepted", "withdrawn", "rejected"}

// terminalStatuses 終了状態：原則として通常更新を受け付けない
var terminalStatuses = map[string]bool{
	"accepted":  true,
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

// Apply 企業への応募を登録する（#1017）
// match の所有権・企業一致を検証し、進行中の応募が既にある場合のみ重複エラーとする。
// 応募作成とマッチの is_applied 更新は同一トランザクションで行い、
// 片方が失敗した場合は両方ロールバックする。
func (s *ApplicationService) Apply(userID, companyID, matchID uint) (*entity.UserApplicationStatus, error) {
	// match の所有権・企業一致チェック（ToggleFavorite と同じパターン: #924）
	match, err := s.matchRepo.FindByID(matchID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, shared.ErrNotFound
		}
		return nil, fmt.Errorf("マッチデータ取得エラー: %w", err)
	}
	if match.UserID != userID {
		return nil, shared.ErrForbidden
	}
	if match.CompanyID != companyID {
		return nil, fmt.Errorf("match_id と company_id が一致しません")
	}

	var app *entity.UserApplicationStatus
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 進行中（終了状態でない）の応募が既にあるか確認
		var existing models.UserApplicationStatus
		findErr := tx.Where("user_id = ? AND company_id = ? AND status NOT IN ?", userID, companyID, terminalStatusList).
			First(&existing).Error
		switch {
		case findErr == nil:
			return shared.ErrDuplicateActiveApplication
		case errors.Is(findErr, gorm.ErrRecordNotFound):
			// 進行中の応募なし。続行
		default:
			return fmt.Errorf("重複チェックエラー: %w", findErr)
		}

		now := time.Now()
		m := &models.UserApplicationStatus{
			UserID:    userID,
			CompanyID: companyID,
			MatchID:   matchID,
			Status:    "applied",
			AppliedAt: &now,
		}
		if createErr := tx.Create(m).Error; createErr != nil {
			// 並行リクエストによる競合はDBのUNIQUE制約（active_dedup_key）で最終防御される
			if isDuplicateEntryErr(createErr) {
				return shared.ErrDuplicateActiveApplication
			}
			return fmt.Errorf("応募登録エラー: %w", createErr)
		}

		// UserCompanyMatch の IsApplied フラグも更新。失敗時は Create ごとロールバックする
		if markErr := tx.Model(&models.UserCompanyMatch{}).Where("id = ?", matchID).Update("is_applied", true).Error; markErr != nil {
			return fmt.Errorf("マッチのis_applied更新エラー: %w", markErr)
		}

		app = mapper.UserApplicationStatusToEntity(m)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return app, nil
}

// isDuplicateEntryErr はMySQLの一意制約違反(Error 1062)かどうかを判定する。
// school_service.go の同名関数と同じ判定ロジック。
func isDuplicateEntryErr(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

// UpdateStatus 選考ステータスを更新する
// isAdmin=true の場合は管理者権限の遷移が許可される（コントローラー側でクライアント入力から isAdmin を取らないこと）
func (s *ApplicationService) UpdateStatus(applicationID uint, userID uint, status, notes string, isAdmin bool) (*entity.UserApplicationStatus, error) {
	if !isValidStatus(status) {
		return nil, fmt.Errorf("invalid_status: 無効なステータス %s", status)
	}

	// 所有権確認（非管理者は自分の応募のみ更新可能）
	app, err := s.appRepo.FindByID(applicationID)
	if err != nil {
		return nil, fmt.Errorf("application_not_found: 応募データが見つかりません: %w", err)
	}
	if !isAdmin && app.UserID != userID {
		return nil, fmt.Errorf("forbidden: 権限がありません")
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

// Withdraw 選考を辞退する（ユーザー本人または管理者。§10.3）
// 遷移表・終了状態チェックは UpdateStatus に委譲する（単一の正とするため）。
func (s *ApplicationService) Withdraw(applicationID, userID uint, isAdmin bool) (*entity.UserApplicationStatus, error) {
	return s.UpdateStatus(applicationID, userID, "withdrawn", "", isAdmin)
}

// Accept 内定を承諾する（§10.4）。offered 以外は application_not_offered を返す。
func (s *ApplicationService) Accept(applicationID, userID uint, isAdmin bool) (*entity.UserApplicationStatus, error) {
	app, err := s.appRepo.FindByID(applicationID)
	if err != nil {
		return nil, fmt.Errorf("application_not_found: 応募データが見つかりません: %w", err)
	}
	if !isAdmin && app.UserID != userID {
		return nil, fmt.Errorf("forbidden: 権限がありません")
	}
	if app.Status != "offered" {
		return nil, fmt.Errorf("application_not_offered: 内定状態でないため承諾できません（現在: %s）", app.Status)
	}
	return s.UpdateStatus(applicationID, userID, "accepted", "", isAdmin)
}

// GetApplicationsByUser ユーザーの応募一覧を取得する
func (s *ApplicationService) GetApplicationsByUser(userID uint) ([]*entity.UserApplicationStatus, error) {
	return s.appRepo.FindByUserID(userID)
}

// ListForAdmin 管理者向けに条件を指定して応募一覧を取得する（§10.5）。
// userID/companyID が 0、status が空文字の場合はその条件を絞り込まない。
func (s *ApplicationService) ListForAdmin(userID, companyID uint, status string) ([]*entity.UserApplicationStatus, error) {
	return s.appRepo.FindAll(userID, companyID, status)
}

// GetCorrelation マッチングスコアと選考通過率の相関データを取得する
func (s *ApplicationService) GetCorrelation(companyID uint) ([]map[string]any, error) {
	if companyID > 0 {
		return s.appRepo.GetCorrelationByCompany(companyID)
	}
	return s.appRepo.GetGlobalCorrelation()
}
