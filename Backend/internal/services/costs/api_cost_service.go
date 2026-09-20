package costs

import (
	"Backend/internal/models"
	openaiPkg "Backend/internal/openai"
	"Backend/internal/repositories"
	"Backend/internal/services/shared"
	"Backend/internal/usagectx"
	"context"
	"encoding/json"
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
	// Web検索系。検索結果が固定トークンとして課金されるため入力トークンが
	// 桁違いに大きい（実測で1コール平均3万トークン）。
	// gpt-4o-mini-search-preview は最長一致が無いと gpt-4o 単価に当たり得るので
	// 明示が必要。残り2つは既定と同値だが、単価の根拠を残すために書いておく。
	"gpt-4o-search-preview":      {2.50, 10.00},
	"gpt-4o-mini-search-preview": {0.15, 0.60},
	"gpt-5-search-api":           {2.50, 10.00},
}

// 音声経路の単価（#1294 DesignDoc §3.4）。
//
// STT は分単位、TTS は文字単位で課金され、トークン単価の表では表せない。
//
// 既定値は 2026-05 時点で確認した公開価格に基づく。価格改定が入っている可能性があるため、
// 実請求額との突き合わせで必ず確認すること（#1193 では旧価格が3年ぶん残っていた）。
// 生の使用量（audio_seconds / characters）を保存しているので、単価が違っていても
// 後から再計算できる。
func sttCostPerMinuteUSD() float64 {
	return shared.GetFloatEnv("STT_COST_PER_MINUTE_USD", 0.006)
}

func ttsCostPer1MCharsUSD() float64 {
	return shared.GetFloatEnv("TTS_COST_PER_1M_CHARS_USD", 15.0)
}

// calculateAudioCost は音声経路のコストを返す。ローカル推論は 0。
func calculateAudioCost(provider string, audioSeconds float64, characters int) float64 {
	if provider != "" && !strings.EqualFold(provider, "openai") {
		return 0
	}
	cost := 0.0
	if audioSeconds > 0 {
		cost += audioSeconds / 60.0 * sttCostPerMinuteUSD()
	}
	if characters > 0 {
		cost += float64(characters) / 1_000_000.0 * ttsCostPer1MCharsUSD()
	}
	return cost
}

// modelPricingOverrideEnv は単価表を再ビルドなしに差し替える env（#1294 DesignDoc §3.4）。
//
//	AI_MODEL_PRICING_JSON={"gpt-4o":[2.5,10.0],"gpt-5.2":[1.25,5.0]}
//
// 価格改定は不定期に起きるのに、テーブルは定数なのでデプロイを待つことになる。
// 実測値が狂うと「ローカル化でいくら減ったか」の判断材料そのものが狂うため、
// 運用側で先に直せるようにする。
const modelPricingOverrideEnv = "AI_MODEL_PRICING_JSON"

func init() {
	applyModelPricingOverride(os.Getenv(modelPricingOverrideEnv))
}

// applyModelPricingOverride は JSON の単価をテーブルへ上書きする。
// 不正な値は無視してログに残す。計測の設定ミスでサーバーを起動不能にしない。
func applyModelPricingOverride(raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	var override map[string][]float64
	if err := json.Unmarshal([]byte(raw), &override); err != nil {
		log.Printf("[APICost] %s を解釈できませんでした（既定の単価表を使います）: %v", modelPricingOverrideEnv, err)
		return
	}
	for model, rates := range override {
		key := strings.ToLower(strings.TrimSpace(model))
		if key == "" || len(rates) != 2 || rates[0] < 0 || rates[1] < 0 {
			log.Printf("[APICost] %s の %q を無視しました（[入力単価, 出力単価] の非負の2要素が必要）", modelPricingOverrideEnv, model)
			continue
		}
		modelPricing[key] = [2]float64{rates[0], rates[1]}
	}
}

// calculateCost は入力/出力トークン数とモデル名からUSDコストを計算する。
//
// provider が openai 以外のときは 0 を返す。ローカル推論は無料であり、
// かつローカルのモデル名（gpt-oss-20b 等）は単価表に無いため
// 未知モデルの既定（gpt-4o 単価）が架空のコストとして記録されてしまう（#1293）。
func calculateCost(provider, model string, promptTokens, completionTokens int) float64 {
	if provider != "" && !strings.EqualFold(provider, "openai") {
		return 0
	}
	lower := strings.ToLower(strings.TrimSpace(model))

	// バージョン付きモデル名（gpt-4o-2024-08-06 等）に対応するため前方一致を使うが、
	// 必ず**最長一致**を取る。
	//
	// 以前は map を range して最初に一致したもので break していた。Go の map の
	// 反復順序はランダムなので、"gpt-4o-mini" が "gpt-4o" の前方一致に当たると
	// 16.7倍高い gpt-4o 単価で記録されていた。しかも実行ごとに結果が変わる。
	// 実測（本番相当DB, 2026-08-13以降）では gpt-4o-mini の 892万入力トークンが
	// $26.32 として記録されており、正しい mini 単価なら $1.79 だった。
	// コスト削減の判断材料が15倍近く狂っていたことになる。
	pricing := [2]float64{2.50, 10.00} // 未知モデルは gpt-4o 単価に倒す（過小評価しない）
	best := ""
	for k := range modelPricing {
		if lower != k && !strings.HasPrefix(lower, k+"-") && !strings.HasPrefix(lower, k+":") {
			continue
		}
		if len(k) > len(best) {
			best = k
		}
	}
	if best != "" {
		pricing = modelPricing[best]
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
	totalCostFn      func() (float64, error)
	modelBreakdownFn func(since time.Time) ([]ModelCostSummary, error)
	notifySlackFn    func(text string) error
	notifyDiscordFn  func(content string) error
}

func NewAPICostService(repo *repositories.APICallLogRepository) *APICostService {
	// OPENAI_COST_ALERT_THRESHOLD_USD を優先、未設定時は旧 API_COST_ALERT_THRESHOLD_USD、最終既定 $40
	threshold := shared.GetFloatEnv("OPENAI_COST_ALERT_THRESHOLD_USD", 0)
	if threshold <= 0 {
		threshold = shared.GetFloatEnv("API_COST_ALERT_THRESHOLD_USD", 40)
	}
	return &APICostService{repo: repo, alertThresholdUSD: threshold}
}

// LogUsage は非同期でAPIコールログをDBに記録する。
//
// provider / via_fallback を残すのは、この表を「OpenAI への課金額」として
// 使えるようにするため（#1293）。フォールバックの USD 上限は via_fallback だけを
// 集計するので、通常の OpenAI 利用（企業検索など）が保険の予算を食わない。
func (s *APICostService) LogUsage(u openaiPkg.Usage) {
	go func() {
		// 音声経路（STT/TTS）はトークンが返らない。秒数・文字数から計算する。
		cost := calculateCost(u.Provider, u.Model, u.PromptTokens, u.CompletionTokens)
		if u.AudioSeconds > 0 || u.Characters > 0 {
			cost = calculateAudioCost(u.Provider, u.AudioSeconds, u.Characters)
		}
		entry := &models.APICallLog{
			Model:            u.Model,
			PromptTokens:     u.PromptTokens,
			CompletionTokens: u.CompletionTokens,
			TotalTokens:      u.PromptTokens + u.CompletionTokens,
			CostUSD:          cost,
			Provider:         u.Provider,
			ViaFallback:      u.ViaFallback,
			CalledAt:         time.Now().UTC(),
			// 配賦軸（#1294）。Feature が空の経路は unknown として残し、
			// 「まだ計測できていない経路」を件数で追えるようにする。
			Feature:        featureOrUnknown(u.Feature),
			UserID:         u.UserID,
			OrganizationID: u.OrganizationID,
			AudioSeconds:   u.AudioSeconds,
			Characters:     u.Characters,
			LatencyMs:      u.LatencyMs,
			CacheHit:       u.CacheHit,
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

// UsageBreakdownSummary は軸ごとの利用量集計（#1294）。
type UsageBreakdownSummary struct {
	Key           string  `json:"key"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
	TotalTokens   int64   `json:"total_tokens"`
	CallCount     int64   `json:"call_count"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	CacheHitCount int64   `json:"cache_hit_count"`
}

// GetUsageBreakdown は指定軸（機能/プロバイダ/モデル/組織）で利用量を集計する（#1294）。
// 未知の軸はエラーにする。呼び出し元のクエリパラメータをそのまま SQL へ渡さないため。
func (s *APICostService) GetUsageBreakdown(ctx context.Context, since time.Time, dim repositories.BreakdownDimension) ([]UsageBreakdownSummary, error) {
	rows, err := s.repo.UsageBreakdown(ctx, since, dim)
	if err != nil {
		return nil, err
	}
	result := make([]UsageBreakdownSummary, len(rows))
	for i, r := range rows {
		result[i] = UsageBreakdownSummary{
			Key:           r.Key,
			TotalCostUSD:  r.TotalCostUSD,
			TotalTokens:   r.TotalTokens,
			CallCount:     r.CallCount,
			AvgLatencyMs:  r.AvgLatencyMs,
			CacheHitCount: r.CacheHitCount,
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

// featureOrUnknown は空の機能名を 'unknown' に寄せる（#1294）。
// DB 側の既定値と一致させ、集計時に空文字と unknown が分かれないようにする。
func featureOrUnknown(feature string) string {
	if feature == "" {
		return usagectx.FeatureUnknown
	}
	return feature
}
