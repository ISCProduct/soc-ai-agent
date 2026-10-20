package costs

import (
	"context"
	"strconv"
	"testing"
	"time"

	"Backend/internal/models"
)

type fakeAIBudgetRepository struct {
	settings map[string]models.AIBudgetSetting
	usage    map[string]float64
	schoolID *uint
	saved    *models.AIBudgetSetting
}

func (r *fakeAIBudgetRepository) FindSetting(_ context.Context, scope string, scopeID uint) (*models.AIBudgetSetting, error) {
	setting, ok := r.settings[budgetTestKey(scope, scopeID)]
	if !ok {
		return nil, nil
	}
	return &setting, nil
}

func (r *fakeAIBudgetRepository) ListSettings(context.Context, int, int) ([]models.AIBudgetSetting, int64, error) {
	return nil, 0, nil
}

func (r *fakeAIBudgetRepository) SaveSetting(_ context.Context, setting *models.AIBudgetSetting) error {
	copy := *setting
	r.saved = &copy
	return nil
}

func (r *fakeAIBudgetRepository) DeleteSetting(context.Context, string, uint) error {
	return nil
}

func (r *fakeAIBudgetRepository) SchoolIDForUser(context.Context, uint) (*uint, error) {
	return r.schoolID, nil
}

func (r *fakeAIBudgetRepository) AcquireBudgetLocks(context.Context, uint) (func() error, error) {
	return func() error { return nil }, nil
}

func (r *fakeAIBudgetRepository) UsageCostSince(_ context.Context, scope string, scopeID uint, since, _ time.Time) (float64, error) {
	period := "month"
	if since.Day() != 1 {
		period = "day"
	}
	return r.usage[budgetTestKey(scope+":"+period, scopeID)], nil
}

func (r *fakeAIBudgetRepository) ScopeExists(context.Context, string, uint) (bool, error) {
	return true, nil
}

func budgetTestKey(scope string, scopeID uint) string {
	return scope + ":" + strconv.FormatUint(uint64(scopeID), 10)
}

func TestAIBudgetStatusEnforcesUserAndSchoolPeriods(t *testing.T) {
	schoolID := uint(7)
	repo := &fakeAIBudgetRepository{
		settings: map[string]models.AIBudgetSetting{
			budgetTestKey(models.AIBudgetScopeUser, 4): {
				DailyLimitUSD: 10, MonthlyLimitUSD: 20,
			},
			budgetTestKey(models.AIBudgetScopeSchool, schoolID): {
				DailyLimitUSD: 30, MonthlyLimitUSD: 40,
			},
		},
		usage: map[string]float64{
			budgetTestKey(models.AIBudgetScopeUser+":day", 4):     3,
			budgetTestKey(models.AIBudgetScopeUser+":month", 4):   20,
			budgetTestKey(models.AIBudgetScopeSchool+":day", 7):   10,
			budgetTestKey(models.AIBudgetScopeSchool+":month", 7): 40,
		},
		schoolID: &schoolID,
	}
	service := NewAIBudgetService(repo)
	service.now = func() time.Time { return time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC) }

	status, err := service.Status(context.Background(), 4)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Allowed {
		t.Fatal("Status().Allowed = true when a monthly budget is exhausted")
	}
	if status.User.Monthly.UsedUSD != 20 || !status.User.Monthly.Exceeded {
		t.Errorf("user monthly status = %+v, want exhausted at $20", status.User.Monthly)
	}
	if status.School == nil || !status.School.Monthly.Exceeded {
		t.Errorf("school monthly status = %+v, want exhausted at $40", status.School)
	}
}

func TestAIBudgetSetLimitAllowsUserMonthlyLimit(t *testing.T) {
	repo := &fakeAIBudgetRepository{}
	service := NewAIBudgetService(repo)

	err := service.SetLimit(context.Background(), models.AIBudgetScopeUser, 4, 5, 50)
	if err != nil {
		t.Fatalf("SetLimit() error = %v", err)
	}
	if repo.saved == nil || repo.saved.MonthlyLimitUSD != 50 {
		t.Fatalf("saved setting = %+v, want user monthly limit $50", repo.saved)
	}
}

func TestValidBudgetLimitMatchesDecimalColumnPrecision(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  bool
	}{
		{"最大値", 999999.999999, true},
		{"最大値超過", 1000000, false},
		{"6桁小数", 1.123456, true},
		{"7桁小数", 1.1234567, false},
		{"7桁目が小さい値", 1.1234560001, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validBudgetLimit(tt.value); got != tt.want {
				t.Errorf("validBudgetLimit(%v) = %t, want %t", tt.value, got, tt.want)
			}
		})
	}
}
