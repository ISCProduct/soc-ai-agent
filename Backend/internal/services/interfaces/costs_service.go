package interfaces

import (
	"Backend/internal/services/costs"
	"time"
)

// APICostService APIコストサービスのインターフェース
type APICostService interface {
	GetCurrentMonthTotal() (float64, error)
	GetModelBreakdown(since time.Time) ([]costs.ModelCostSummary, error)
	GetDailyCosts(nDays int) ([]costs.DailyCostSummary, error)
	GetMonthlyCosts(nMonths int) ([]costs.MonthlyCostSummary, error)
}

// RealtimeUsageService リアルタイム使用量サービスのインターフェース
type RealtimeUsageService interface {
	SessionDurationMinutes() int
	CurrentMonthTotalCost() (float64, error)
	CurrentActiveCount() (int64, error)
	GetUserBreakdown(days int, limit int) ([]costs.RealtimeUserSummary, error)
	GetDailyUsage(nDays int) ([]costs.RealtimeDailySummary, error)
	GetMonthlyUsage(nMonths int) ([]costs.RealtimeMonthlySummary, error)
}

// CompanySearchBudgetService 企業 Search 月次予算のインターフェース（#587）
type CompanySearchBudgetService interface {
	Status() (costs.CompanySearchBudgetStatus, error)
	AllowSearch() error
}
