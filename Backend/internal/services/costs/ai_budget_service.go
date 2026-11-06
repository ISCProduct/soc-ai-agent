package costs

import (
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"context"
	"errors"
	"math"
	"time"
)

// aiBudgetLocation は日次・月次の窓を切る基準。利用者も学校も日本にいるので、
// 授業の途中や月初の朝に窓が切り替わらないよう日本時間の暦に合わせる。
var aiBudgetLocation = time.FixedZone("JST", 9*60*60)

var (
	ErrInvalidAIBudgetScope   = errors.New("invalid AI budget scope")
	ErrInvalidAIBudgetLimit   = errors.New("AI budget limits must be finite non-negative amounts")
	ErrAIBudgetTargetNotFound = errors.New("AI budget target not found")
)

type AIBudgetPeriodStatus struct {
	LimitUSD     float64 `json:"limit_usd"`
	UsedUSD      float64 `json:"used_usd"`
	RemainingUSD float64 `json:"remaining_usd"`
	Exceeded     bool    `json:"exceeded"`
}

type AIBudgetScopeStatus struct {
	Scope   string               `json:"scope"`
	ScopeID uint                 `json:"scope_id"`
	Daily   AIBudgetPeriodStatus `json:"daily"`
	Monthly AIBudgetPeriodStatus `json:"monthly"`
}

type AIBudgetStatus struct {
	Date    string               `json:"date"`
	User    AIBudgetScopeStatus  `json:"user"`
	School  *AIBudgetScopeStatus `json:"school,omitempty"`
	Allowed bool                 `json:"allowed"`
}

type aiBudgetRepository interface {
	FindSetting(context.Context, string, uint) (*models.AIBudgetSetting, error)
	ListSettings(context.Context, int, int) ([]models.AIBudgetSetting, int64, error)
	SaveSetting(context.Context, *models.AIBudgetSetting) error
	DeleteSetting(context.Context, string, uint) error
	SchoolIDForUser(context.Context, uint) (*uint, error)
	UsageCosts(ctx context.Context, scope string, scopeID uint, dayStart, monthStart time.Time, realtimeRatePerMinute float64) (daily, monthly float64, err error)
	ScopeExists(context.Context, string, uint) (bool, error)
}

type AIBudgetService struct {
	repo                  aiBudgetRepository
	now                   func() time.Time
	realtimeRatePerMinute float64
}

func NewAIBudgetService(repo aiBudgetRepository) *AIBudgetService {
	return &AIBudgetService{
		repo:                  repo,
		now:                   time.Now,
		realtimeRatePerMinute: shared.GetFloatEnv("INTERVIEW_COST_PER_MIN_USD", 0.18),
	}
}

func (s *AIBudgetService) Status(ctx context.Context, userID uint) (AIBudgetStatus, error) {
	if s == nil || s.repo == nil || userID == 0 {
		return AIBudgetStatus{}, errors.New("AI budget service is not configured")
	}
	now := s.now().In(aiBudgetLocation)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, aiBudgetLocation)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, aiBudgetLocation)
	userStatus, err := s.scopeStatus(ctx, models.AIBudgetScopeUser, userID, dayStart, monthStart)
	if err != nil {
		return AIBudgetStatus{}, err
	}
	status := AIBudgetStatus{
		Date:    now.Format("2006-01-02"),
		User:    userStatus,
		Allowed: !userStatus.Daily.Exceeded && !userStatus.Monthly.Exceeded,
	}
	schoolID, err := s.repo.SchoolIDForUser(ctx, userID)
	if err != nil {
		return AIBudgetStatus{}, err
	}
	if schoolID != nil && *schoolID > 0 {
		schoolStatus, err := s.scopeStatus(ctx, models.AIBudgetScopeSchool, *schoolID, dayStart, monthStart)
		if err != nil {
			return AIBudgetStatus{}, err
		}
		status.School = &schoolStatus
		status.Allowed = status.Allowed && !schoolStatus.Daily.Exceeded && !schoolStatus.Monthly.Exceeded
	}
	return status, nil
}

func (s *AIBudgetService) scopeStatus(
	ctx context.Context,
	scope string,
	scopeID uint,
	dayStart, monthStart time.Time,
) (AIBudgetScopeStatus, error) {
	setting, err := s.repo.FindSetting(ctx, scope, scopeID)
	if err != nil {
		return AIBudgetScopeStatus{}, err
	}
	status := AIBudgetScopeStatus{Scope: scope, ScopeID: scopeID}
	if setting == nil {
		return status, nil
	}
	status.Daily.LimitUSD = setting.DailyLimitUSD
	status.Monthly.LimitUSD = setting.MonthlyLimitUSD
	if setting.DailyLimitUSD <= 0 && setting.MonthlyLimitUSD <= 0 {
		return status, nil
	}
	daily, monthly, err := s.repo.UsageCosts(ctx, scope, scopeID, dayStart, monthStart, s.realtimeRatePerMinute)
	if err != nil {
		return AIBudgetScopeStatus{}, err
	}
	status.Daily = budgetPeriodStatus(setting.DailyLimitUSD, daily)
	status.Monthly = budgetPeriodStatus(setting.MonthlyLimitUSD, monthly)
	return status, nil
}

// budgetPeriodStatus は期間の状態を作る。上限 0 は「上限なし」で、超過にしない。
func budgetPeriodStatus(limit, used float64) AIBudgetPeriodStatus {
	if limit <= 0 {
		return AIBudgetPeriodStatus{}
	}
	return AIBudgetPeriodStatus{
		LimitUSD:     limit,
		UsedUSD:      used,
		RemainingUSD: remainingBudget(limit, used),
		Exceeded:     used >= limit,
	}
}

func remainingBudget(limit, used float64) float64 {
	if used >= limit {
		return 0
	}
	return limit - used
}

func (s *AIBudgetService) ListSettings(ctx context.Context, limit, offset int) ([]models.AIBudgetSetting, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, errors.New("AI budget service is not configured")
	}
	return s.repo.ListSettings(ctx, limit, offset)
}

func (s *AIBudgetService) SetLimit(
	ctx context.Context,
	scope string,
	scopeID uint,
	dailyLimitUSD, monthlyLimitUSD float64,
) error {
	if s == nil || s.repo == nil {
		return errors.New("AI budget service is not configured")
	}
	if scope != models.AIBudgetScopeUser && scope != models.AIBudgetScopeSchool {
		return ErrInvalidAIBudgetScope
	}
	if scopeID == 0 || !validBudgetLimit(dailyLimitUSD) || !validBudgetLimit(monthlyLimitUSD) {
		return ErrInvalidAIBudgetLimit
	}
	exists, err := s.repo.ScopeExists(ctx, scope, scopeID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrAIBudgetTargetNotFound
	}
	return s.repo.SaveSetting(ctx, &models.AIBudgetSetting{
		Scope:           scope,
		ScopeID:         scopeID,
		DailyLimitUSD:   dailyLimitUSD,
		MonthlyLimitUSD: monthlyLimitUSD,
	})
}

func (s *AIBudgetService) DeleteLimit(ctx context.Context, scope string, scopeID uint) error {
	if s == nil || s.repo == nil {
		return errors.New("AI budget service is not configured")
	}
	if scope != models.AIBudgetScopeUser && scope != models.AIBudgetScopeSchool {
		return ErrInvalidAIBudgetScope
	}
	if scopeID == 0 {
		return ErrInvalidAIBudgetLimit
	}
	return s.repo.DeleteSetting(ctx, scope, scopeID)
}

func validBudgetLimit(value float64) bool {
	if value < 0 || value > 999999.999999 {
		return false
	}
	rounded := math.Round(value*1_000_000) / 1_000_000
	return value == rounded
}
