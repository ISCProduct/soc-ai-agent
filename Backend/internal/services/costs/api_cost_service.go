package costs

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/shared"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// モデル別単価テーブル (USD per 1M tokens)
var modelPricing = map[string][2]float64{
	// input, output per 1M tokens
	"gpt-4o":                 {2.50, 10.00},
	"gpt-4o-mini":            {0.15, 0.60},
	"gpt-4-turbo":            {10.00, 30.00},
	"gpt-4":                  {30.00, 60.00},
	"gpt-3.5-turbo":          {0.50, 1.50},
	"gpt-5.2":                {2.50, 10.00}, // treated as gpt-4o class
	"o1":                     {15.00, 60.00},
	"o1-mini":                {3.00, 12.00},
	"o3":                     {10.00, 40.00},
	"text-embedding-3-small": {0.02, 0.02},
	"text-embedding-3-large": {0.13, 0.13},
}

// calculateCost は入力/出力トークン数とモデル名からUSDコストを計算する
func calculateCost(model string, promptTokens, completionTokens int) float64 {
	lower := strings.ToLower(strings.TrimSpace(model))

	// prefix match for versioned model names (e.g. gpt-4o-2024-08-06)
	var pricing [2]float64
	var found bool
	for k, v := range modelPricing {
		if lower == k || strings.HasPrefix(lower, k+"-") || strings.HasPrefix(lower, k+":") {
			pricing = v
			found = true
			break
		}
	}
	if !found {
		// Default to gpt-4o pricing
		pricing = [2]float64{2.50, 10.00}
	}

	inputCost := float64(promptTokens) * pricing[0] / 1_000_000
	outputCost := float64(completionTokens) * pricing[1] / 1_000_000
	return inputCost + outputCost
}

// APICostService はAPIコスト記録・集計・月次閾値アラートを担当する
type APICostService struct {
	repo              *repositories.APICallLogRepository
	alertThresholdUSD float64 // 月次閾値（既定 $40）

	mu               sync.Mutex
	lastAlertMonthID string // "2006-01" UTC。同一月は1回のみ通知

	// テスト用フック（nil なら本番実装を使う）
	totalCostFn     func() (float64, error)
	modelBreakdownFn func(since time.Time) ([]ModelCostSummary, error)
	notifySlackFn   func(text string) error
	notifyDiscordFn func(content string) error
}

func NewAPICostService(repo *repositories.APICallLogRepository) *APICostService {
	// OPENAI_COST_ALERT_THRESHOLD_USD を優先、未設定時は旧 API_COST_ALERT_THRESHOLD_USD、最終既定 $40
	threshold := shared.GetFloatEnv("OPENAI_COST_ALERT_THRESHOLD_USD", 0)
	if threshold <= 0 {
		threshold = shared.GetFloatEnv("API_COST_ALERT_THRESHOLD_USD", 40)
	}
	return &APICostService{repo: repo, alertThresholdUSD: threshold}
}

// LogCall は非同期でAPIコールログをDBに記録する
func (s *APICostService) LogCall(model string, promptTokens, completionTokens int) {
	go func() {
		cost := calculateCost(model, promptTokens, completionTokens)
		entry := &models.APICallLog{
			Model:            model,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
			CostUSD:          cost,
			CalledAt:         time.Now().UTC(),
		}
		if err := s.repo.Create(entry); err != nil {
			log.Printf("[APICost] failed to log: %v", err)
			return
		}
		s.checkAndNotifyThreshold()
	}()
}

// checkAndNotifyThreshold は当月（UTC）累計が閾値を超えたら Slack/Discord に通知する。
// 同一月は1回のみ。webhook 未設定時はログのみで落ちない。
func (s *APICostService) checkAndNotifyThreshold() {
	totalFn := s.totalCostFn
	if totalFn == nil {
		totalFn = s.GetCurrentMonthTotal
	}
	total, err := totalFn()
	if err != nil {
		return
	}
	s.NotifyIfMonthCostExceeded(total, time.Now().UTC().Format("2006-01"))
}

// NotifyIfMonthCostExceeded は閾値判定・月次デデュープ・通知送信を行う（テスト可能）。
// 通知を送った場合 true。
func (s *APICostService) NotifyIfMonthCostExceeded(total float64, monthID string) bool {
	// 「超過」は閾値より大きい場合のみ（ちょうど $40 では通知しない）
	if total <= s.alertThresholdUSD {
		return false
	}

	s.mu.Lock()
	if s.lastAlertMonthID == monthID {
		s.mu.Unlock()
		return false
	}
	s.lastAlertMonthID = monthID
	s.mu.Unlock()

	subject := fmt.Sprintf("[SOC AI] OpenAI API月次コスト閾値超過 (%s)", monthID)
	body := fmt.Sprintf(
		"OpenAI API monthly cost exceeded threshold.\nmonth=%s\ntotal_usd=%.4f\nthreshold_usd=%.2f\n",
		monthID, total, s.alertThresholdUSD,
	)
	if breakdown := s.formatModelBreakdown(monthID); breakdown != "" {
		body += "models:\n" + breakdown
	}
	log.Printf("[APICost] ALERT %s", strings.ReplaceAll(body, "\n", " "))

	text := subject + "\n" + body
	s.sendSlackAlert(text)
	s.sendDiscordAlert(text)
	return true
}

func (s *APICostService) formatModelBreakdown(monthID string) string {
	t, err := time.ParseInLocation("2006-01", monthID, time.UTC)
	if err != nil {
		return ""
	}
	since := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	fn := s.modelBreakdownFn
	if fn == nil {
		fn = s.GetModelBreakdown
	}
	rows, err := fn(since)
	if err != nil || len(rows) == 0 {
		return ""
	}
	const maxModels = 5
	var b strings.Builder
	for i, r := range rows {
		if i >= maxModels {
			b.WriteString(fmt.Sprintf("  ... and %d more\n", len(rows)-maxModels))
			break
		}
		b.WriteString(fmt.Sprintf("  - %s: $%.4f (%d calls)\n", r.Model, r.TotalCostUSD, r.CallCount))
	}
	return b.String()
}

func (s *APICostService) sendSlackAlert(text string) {
	if s.notifySlackFn != nil {
		if err := s.notifySlackFn(text); err != nil {
			log.Printf("[APICost] slack alert failed: %v", err)
		}
		return
	}
	webhook := strings.TrimSpace(os.Getenv("OPENAI_COST_ALERT_SLACK_WEBHOOK_URL"))
	if webhook == "" {
		webhook = strings.TrimSpace(os.Getenv("REALTIME_ALERT_SLACK_WEBHOOK_URL"))
	}
	if webhook == "" {
		return
	}
	if err := postSlackAlert(webhook, text); err != nil {
		log.Printf("[APICost] slack alert failed: %v", err)
	}
}

func (s *APICostService) sendDiscordAlert(content string) {
	if s.notifyDiscordFn != nil {
		if err := s.notifyDiscordFn(content); err != nil {
			log.Printf("[APICost] discord alert failed: %v", err)
		}
		return
	}
	webhook := strings.TrimSpace(os.Getenv("OPENAI_COST_ALERT_DISCORD_WEBHOOK_URL"))
	if webhook == "" {
		return
	}
	if err := postDiscordAlert(webhook, content); err != nil {
		log.Printf("[APICost] discord alert failed: %v", err)
	}
}

type DailyCostSummary struct {
	Date         string  `json:"date"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	TotalTokens  int64   `json:"total_tokens"`
	CallCount    int64   `json:"call_count"`
}

type MonthlyCostSummary struct {
	Month        string  `json:"month"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	TotalTokens  int64   `json:"total_tokens"`
	CallCount    int64   `json:"call_count"`
}

type ModelCostSummary struct {
	Model        string  `json:"model"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	TotalTokens  int64   `json:"total_tokens"`
	CallCount    int64   `json:"call_count"`
}

func (s *APICostService) GetDailyCosts(nDays int) ([]DailyCostSummary, error) {
	rows, err := s.repo.DailyCosts(nDays)
	if err != nil {
		return nil, err
	}
	result := make([]DailyCostSummary, len(rows))
	for i, r := range rows {
		result[i] = DailyCostSummary{
			Date:         r.Date,
			TotalCostUSD: r.TotalCostUSD,
			TotalTokens:  r.TotalTokens,
			CallCount:    r.CallCount,
		}
	}
	return result, nil
}

func (s *APICostService) GetMonthlyCosts(nMonths int) ([]MonthlyCostSummary, error) {
	rows, err := s.repo.MonthlyCosts(nMonths)
	if err != nil {
		return nil, err
	}
	result := make([]MonthlyCostSummary, len(rows))
	for i, r := range rows {
		result[i] = MonthlyCostSummary{
			Month:        r.Month,
			TotalCostUSD: r.TotalCostUSD,
			TotalTokens:  r.TotalTokens,
			CallCount:    r.CallCount,
		}
	}
	return result, nil
}

func (s *APICostService) GetModelBreakdown(since time.Time) ([]ModelCostSummary, error) {
	rows, err := s.repo.ModelBreakdown(since)
	if err != nil {
		return nil, err
	}
	result := make([]ModelCostSummary, len(rows))
	for i, r := range rows {
		result[i] = ModelCostSummary{
			Model:        r.Model,
			TotalCostUSD: r.TotalCostUSD,
			TotalTokens:  r.TotalTokens,
			CallCount:    r.CallCount,
		}
	}
	return result, nil
}

func (s *APICostService) GetCurrentMonthTotal() (float64, error) {
	now := time.Now().UTC()
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return s.repo.TotalCostSince(since)
}

// AlertThresholdUSD は設定済みの月次アラート閾値を返す（テスト/管理用）。
func (s *APICostService) AlertThresholdUSD() float64 {
	return s.alertThresholdUSD
}

// SetAlertHooksForTest は単体テスト用に通知フックと閾値を差し替える。
func (s *APICostService) SetAlertHooksForTest(
	threshold float64,
	slackFn func(text string) error,
	discordFn func(content string) error,
) {
	s.alertThresholdUSD = threshold
	s.notifySlackFn = slackFn
	s.notifyDiscordFn = discordFn
	s.modelBreakdownFn = func(time.Time) ([]ModelCostSummary, error) {
		return nil, nil
	}
}
