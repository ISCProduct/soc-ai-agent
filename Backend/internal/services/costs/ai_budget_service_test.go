package costs

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"
)

type fakeAIBudgetUsage struct {
	daily   float64
	monthly float64
}

type fakeAIBudgetRepository struct {
	settings   map[string]models.AIBudgetSetting
	usage      map[string]fakeAIBudgetUsage
	schoolID   *uint
	saved      *models.AIBudgetSetting
	exists     bool
	dayStart   time.Time
	monthStart time.Time
	usageCalls int
	list       []models.AIBudgetSetting
	batchCalls map[string][]uint
}

func budgetTestKey(scope string, scopeID uint) string {
	return scope + ":" + strconv.FormatUint(uint64(scopeID), 10)
}

func (r *fakeAIBudgetRepository) FindSetting(_ context.Context, scope string, scopeID uint) (*models.AIBudgetSetting, error) {
	setting, ok := r.settings[budgetTestKey(scope, scopeID)]
	if !ok {
		return nil, nil
	}
	return &setting, nil
}

func (r *fakeAIBudgetRepository) ListSettings(context.Context, int, int) ([]models.AIBudgetSetting, int64, error) {
	return r.list, int64(len(r.list)), nil
}

func (r *fakeAIBudgetRepository) UsageCostsByScope(
	_ context.Context, scope string, scopeIDs []uint, _, _ time.Time, _ float64,
) (map[uint]repositories.AIBudgetUsage, error) {
	if r.batchCalls == nil {
		r.batchCalls = map[string][]uint{}
	}
	r.batchCalls[scope] = append(r.batchCalls[scope], scopeIDs...)
	result := map[uint]repositories.AIBudgetUsage{}
	for _, id := range scopeIDs {
		if usage, ok := r.usage[budgetTestKey(scope, id)]; ok {
			result[id] = repositories.AIBudgetUsage{Daily: usage.daily, Monthly: usage.monthly}
		}
	}
	return result, nil
}

func (r *fakeAIBudgetRepository) SaveSetting(_ context.Context, setting *models.AIBudgetSetting) error {
	saved := *setting
	r.saved = &saved
	return nil
}

func (r *fakeAIBudgetRepository) DeleteSetting(context.Context, string, uint) error {
	return nil
}

func (r *fakeAIBudgetRepository) SchoolIDForUser(context.Context, uint) (*uint, error) {
	return r.schoolID, nil
}

func (r *fakeAIBudgetRepository) UsageCosts(
	_ context.Context, scope string, scopeID uint, dayStart, monthStart time.Time, _ float64,
) (float64, float64, error) {
	r.usageCalls++
	r.dayStart, r.monthStart = dayStart, monthStart
	usage := r.usage[budgetTestKey(scope, scopeID)]
	return usage.daily, usage.monthly, nil
}

func (r *fakeAIBudgetRepository) ScopeExists(context.Context, string, uint) (bool, error) {
	return r.exists, nil
}

func TestAIBudgetStatus(t *testing.T) {
	school := uint(3)
	userKey := budgetTestKey(models.AIBudgetScopeUser, 1)
	schoolKey := budgetTestKey(models.AIBudgetScopeSchool, school)
	tests := []struct {
		name        string
		settings    map[string]models.AIBudgetSetting
		usage       map[string]fakeAIBudgetUsage
		wantAllowed bool
		wantCalls   int
	}{
		{name: "設定なしなら集計せず許可", wantAllowed: true},
		{
			name:        "枠内",
			settings:    map[string]models.AIBudgetSetting{userKey: {DailyLimitUSD: 1, MonthlyLimitUSD: 10}},
			usage:       map[string]fakeAIBudgetUsage{userKey: {daily: 0.5, monthly: 5}},
			wantAllowed: true, wantCalls: 1,
		},
		{
			name:      "利用者の日次だけ超過",
			settings:  map[string]models.AIBudgetSetting{userKey: {DailyLimitUSD: 1, MonthlyLimitUSD: 10}},
			usage:     map[string]fakeAIBudgetUsage{userKey: {daily: 1, monthly: 5}},
			wantCalls: 1,
		},
		{
			name:      "利用者の月次だけ超過",
			settings:  map[string]models.AIBudgetSetting{userKey: {DailyLimitUSD: 1, MonthlyLimitUSD: 10}},
			usage:     map[string]fakeAIBudgetUsage{userKey: {daily: 0.5, monthly: 10}},
			wantCalls: 1,
		},
		{
			name:      "学校の日次だけ超過",
			settings:  map[string]models.AIBudgetSetting{schoolKey: {DailyLimitUSD: 50}},
			usage:     map[string]fakeAIBudgetUsage{schoolKey: {daily: 50}},
			wantCalls: 1,
		},
		{
			name:      "学校の月次だけ超過",
			settings:  map[string]models.AIBudgetSetting{schoolKey: {MonthlyLimitUSD: 500}},
			usage:     map[string]fakeAIBudgetUsage{schoolKey: {monthly: 500}},
			wantCalls: 1,
		},
		{
			name:        "上限0は上限なし",
			settings:    map[string]models.AIBudgetSetting{userKey: {DailyLimitUSD: 0, MonthlyLimitUSD: 0}},
			usage:       map[string]fakeAIBudgetUsage{userKey: {daily: 100, monthly: 100}},
			wantAllowed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeAIBudgetRepository{settings: tt.settings, usage: tt.usage, schoolID: &school}
			service := NewAIBudgetService(repo)
			status, err := service.Status(context.Background(), 1)
			if err != nil {
				t.Fatalf("Status returned error: %v", err)
			}
			if status.Allowed != tt.wantAllowed || repo.usageCalls != tt.wantCalls {
				t.Fatalf("allowed=%v calls=%d, want %v %d", status.Allowed, repo.usageCalls, tt.wantAllowed, tt.wantCalls)
			}
		})
	}
}

// 日次・月次の窓は日本時間の暦に合わせる。授業中や月初の朝に窓がずれないようにするため。
func TestAIBudgetStatus_窓は日本時間の暦で切る(t *testing.T) {
	userKey := budgetTestKey(models.AIBudgetScopeUser, 1)
	tests := []struct {
		name           string
		now            time.Time
		wantDate       string
		wantDayStart   time.Time
		wantMonthStart time.Time
	}{
		{
			name:           "UTCでは前日でも日本時間の日付を使う",
			now:            time.Date(2026, 11, 5, 16, 0, 0, 0, time.UTC),
			wantDate:       "2026-11-06",
			wantDayStart:   time.Date(2026, 11, 5, 15, 0, 0, 0, time.UTC),
			wantMonthStart: time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC),
		},
		{
			name:           "UTCでは前月でも日本時間の月を使う",
			now:            time.Date(2026, 10, 31, 16, 0, 0, 0, time.UTC),
			wantDate:       "2026-11-01",
			wantDayStart:   time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC),
			wantMonthStart: time.Date(2026, 10, 31, 15, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeAIBudgetRepository{settings: map[string]models.AIBudgetSetting{userKey: {DailyLimitUSD: 1}}}
			service := NewAIBudgetService(repo)
			service.now = func() time.Time { return tt.now }
			status, err := service.Status(context.Background(), 1)
			if err != nil {
				t.Fatalf("Status returned error: %v", err)
			}
			if status.Date != tt.wantDate {
				t.Errorf("date = %s, want %s", status.Date, tt.wantDate)
			}
			if !repo.dayStart.Equal(tt.wantDayStart) || !repo.monthStart.Equal(tt.wantMonthStart) {
				t.Errorf("dayStart=%v monthStart=%v", repo.dayStart.UTC(), repo.monthStart.UTC())
			}
		})
	}
}

func TestAIBudgetSetLimit(t *testing.T) {
	tests := []struct {
		name    string
		scope   string
		scopeID uint
		daily   float64
		monthly float64
		exists  bool
		wantErr error
	}{
		{name: "保存できる", scope: models.AIBudgetScopeUser, scopeID: 1, daily: 1.5, monthly: 20, exists: true},
		{name: "不正なスコープ", scope: "tenant", scopeID: 1, exists: true, wantErr: ErrInvalidAIBudgetScope},
		{name: "対象IDが0", scope: models.AIBudgetScopeUser, exists: true, wantErr: ErrInvalidAIBudgetLimit},
		{name: "負の上限", scope: models.AIBudgetScopeUser, scopeID: 1, daily: -1, exists: true, wantErr: ErrInvalidAIBudgetLimit},
		{name: "精度を超える上限", scope: models.AIBudgetScopeUser, scopeID: 1, daily: 0.0000001, exists: true, wantErr: ErrInvalidAIBudgetLimit},
		{name: "列に収まらない上限", scope: models.AIBudgetScopeUser, scopeID: 1, monthly: 1000000, exists: true, wantErr: ErrInvalidAIBudgetLimit},
		{name: "存在しない対象", scope: models.AIBudgetScopeSchool, scopeID: 9, wantErr: ErrAIBudgetTargetNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeAIBudgetRepository{exists: tt.exists}
			err := NewAIBudgetService(repo).SetLimit(context.Background(), tt.scope, tt.scopeID, tt.daily, tt.monthly)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && (repo.saved == nil || repo.saved.DailyLimitUSD != tt.daily || repo.saved.MonthlyLimitUSD != tt.monthly) {
				t.Fatalf("saved = %+v", repo.saved)
			}
		})
	}
}

// 一覧の利用額は、設定1件ごとではなく対象の種類ごとに1回だけ読む。
func TestAIBudgetListSettingsWithUsage_種類ごとにまとめて読む(t *testing.T) {
	repo := &fakeAIBudgetRepository{
		list: []models.AIBudgetSetting{
			{Scope: models.AIBudgetScopeSchool, ScopeID: 3, DailyLimitUSD: 50},
			{Scope: models.AIBudgetScopeUser, ScopeID: 1, DailyLimitUSD: 1},
			{Scope: models.AIBudgetScopeUser, ScopeID: 2, MonthlyLimitUSD: 10},
		},
		usage: map[string]fakeAIBudgetUsage{
			budgetTestKey(models.AIBudgetScopeSchool, 3): {daily: 20, monthly: 100},
			budgetTestKey(models.AIBudgetScopeUser, 1):   {daily: 0.5, monthly: 3},
		},
	}
	rows, total, err := NewAIBudgetService(repo).ListSettingsWithUsage(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("ListSettingsWithUsage returned error: %v", err)
	}
	if total != 3 || len(rows) != 3 {
		t.Fatalf("total=%d rows=%d", total, len(rows))
	}
	if repo.usageCalls != 0 || len(repo.batchCalls) != 2 ||
		len(repo.batchCalls[models.AIBudgetScopeUser]) != 2 || len(repo.batchCalls[models.AIBudgetScopeSchool]) != 1 {
		t.Fatalf("usageCalls=%d batchCalls=%v", repo.usageCalls, repo.batchCalls)
	}
	if rows[0].DailyUsedUSD != 20 || rows[1].MonthlyUsedUSD != 3 || rows[2].DailyUsedUSD != 0 {
		t.Fatalf("rows=%+v", rows)
	}
}
