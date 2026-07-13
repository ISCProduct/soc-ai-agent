package interfaces

import (
	"Backend/internal/services"
	"time"
)

// APICostService APIコストサービスのインターフェース
type APICostService interface {
	GetCurrentMonthTotal() (float64, error)
	GetModelBreakdown(since time.Time) ([]services.ModelCostSummary, error)
	GetDailyCosts(nDays int) ([]services.DailyCostSummary, error)
	GetMonthlyCosts(nMonths int) ([]services.MonthlyCostSummary, error)
}

// RealtimeUsageService リアルタイム使用量サービスのインターフェース
type RealtimeUsageService interface {
	SessionDurationMinutes() int
	CurrentMonthTotalCost() (float64, error)
	CurrentActiveCount() (int64, error)
	GetUserBreakdown(days int, limit int) ([]services.RealtimeUserSummary, error)
	GetDailyUsage(nDays int) ([]services.RealtimeDailySummary, error)
	GetMonthlyUsage(nMonths int) ([]services.RealtimeMonthlySummary, error)
}

// CompanySearchBudgetService 企業 Search 月次予算のインターフェース（#587）
type CompanySearchBudgetService interface {
	Status() (services.CompanySearchBudgetStatus, error)
	AllowSearch() error
}
