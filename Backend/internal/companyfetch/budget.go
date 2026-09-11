package companyfetch

import "errors"

// ErrSearchBudgetExceeded は月次企業 Search 上限に達したときに返す。
var ErrSearchBudgetExceeded = errors.New("company search monthly budget exceeded")

// SearchBudget は企業 Search 実行前の予算チェック。
type SearchBudget interface {
	AllowSearch() error
}
