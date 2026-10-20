package costs

import (
	"Backend/internal/models"
	"context"
	"errors"
	"math"
	"time"
)

var ErrAIBudgetExceeded = errors.New("AI budget exceeded")

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
	AcquireBudgetLocks(context.Context, uint) (func() error, error)
	UsageCostSince(context.Context, string, uint, time.Time, time.Time) (float64, error)
	ScopeExists(context.Context, string, uint) (bool, error)
}

type AIBudgetService struct {
	repo aiBudgetRepository
	now  func() time.Time
}

func NewAIBudgetService(repo aiBudgetRepository) *AIBudgetService {
	return &AIBudgetService{repo: repo, now: time.Now}
}

func (s *AIBudgetService) Status(ctx context.Context, userID uint) (AIBudgetStatus, error) {
	if s == nil || s.repo == nil || userID == 0 {
		return AIBudgetStatus{}, errors.New("AI budget service is not configured")
	}
	now := s.now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	userStatus, err := s.scopeStatus(ctx, models.AIBudgetScopeUser, userID, dayStart, monthStart, now)
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
		schoolStatus, err := s.scopeStatus(ctx, models.AIBudgetScopeSchool, *schoolID, dayStart, monthStart, now)
		if err != nil {
			return AIBudgetStatus{}, err
		}
		status.School = &schoolStatus
		status.Allowed = status.Allowed && !schoolStatus.Daily.Exceeded && !schoolStatus.Monthly.Exceeded
	}
	return status, nil
}

func (s *AIBudgetService) Acquire(ctx context.Context, userID uint) (func() error, error) {
	if s == nil || s.repo == nil || userID == 0 {
		return nil, errors.New("AI budget service is not configured")
	}
	return s.repo.AcquireBudgetLocks(ctx, userID)
}

func (s *AIBudgetService) scopeStatus(
	ctx context.Context,
	scope string,
	scopeID uint,
	dayStart, monthStart, now time.Time,
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
	if setting.DailyLimitUSD > 0 {
		status.Daily.UsedUSD, err = s.repo.UsageCostSince(ctx, scope, scopeID, dayStart, now)
		if err != nil {
			return AIBudgetScopeStatus{}, err
		}
		status.Daily.RemainingUSD = remainingBudget(setting.DailyLimitUSD, status.Daily.UsedUSD)
		status.Daily.Exceeded = status.Daily.UsedUSD >= setting.DailyLimitUSD
	}
	if setting.MonthlyLimitUSD > 0 {
		status.Monthly.UsedUSD, err = s.repo.UsageCostSince(ctx, scope, scopeID, monthStart, now)
		if err != nil {
			return AIBudgetScopeStatus{}, err
		}
		status.Monthly.RemainingUSD = remainingBudget(setting.MonthlyLimitUSD, status.Monthly.UsedUSD)
		status.Monthly.Exceeded = status.Monthly.UsedUSD >= setting.MonthlyLimitUSD
	}
	return status, nil
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
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 999999.999999 {
		return false
	}
	rounded := math.Round(value*1_000_000) / 1_000_000
	return value == rounded
}
