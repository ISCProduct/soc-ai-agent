package costs

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// defaultDailyHardLimitUSD / defaultMonthlyHardLimitUSD は OpenAI へのフォールバックを
	// 打ち切る既定の上限。学生向けサービスとして「ローカル障害で全トラフィックが
	// 従量課金へ流れる」事故を防ぐための保険なので、意図的に低く設定している(#1293)。
	defaultDailyHardLimitUSD   = 2.0
	defaultMonthlyHardLimitUSD = 20.0

	// costCacheTTL は上限判定用のコスト集計をキャッシュする時間。
	// ponytail: 固定30秒。障害時はフォールバックが連続するため、毎回集計すると
	// DB を叩きすぎる。上限を秒単位で厳密に守る必要が出たら短縮する。
	costCacheTTL = 30 * time.Second
)

// errNoCostRepo は集計リポジトリが未注入であることを表す。
var errNoCostRepo = errors.New("cost repository is not configured")

// OpenAIFallbackGuard は OpenAI へのフォールバックを日次/月次のUSD上限と
// 分間リクエスト数で制限する（#1293）。
//
// ローカル推論先の障害時に無条件で OpenAI へ流すと、障害が続く間ずっと
// 全トラフィックが従量課金へ移り高額請求になる。上限に達したら
// 「Local AI unavailable」として縮退させ、サービス全体は止めない。
type OpenAIFallbackGuard struct {
	repo costRepo
	now  func() time.Time

	mu sync.Mutex
	// 分間レート
	minuteWindow time.Time
	minuteCount  int
	// コスト集計のキャッシュ
	cachedAt      time.Time
	cachedDaily   float64
	cachedMonthly float64
}

// costRepo は上限判定に必要な集計だけを切り出したインターフェース（テスト用に差し替える）。
type costRepo interface {
	TotalCostSince(since time.Time) (float64, error)
}

func NewOpenAIFallbackGuard(repo costRepo) *OpenAIFallbackGuard {
	return &OpenAIFallbackGuard{repo: repo, now: time.Now}
}

// AllowFallback は OpenAI へのフォールバックを許可するかを返す。
// 拒否する場合は理由を返す（ログに出して運用で気づけるようにする）。
func (g *OpenAIFallbackGuard) AllowFallback() (bool, string) {
	if g == nil {
		return false, "guard not configured"
	}
	if !envFlagEnabled("OPENAI_FALLBACK_ENABLED") {
		return false, "OPENAI_FALLBACK_ENABLED=false"
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now().UTC()

	// 1) 分間レート制限（0 or 未設定なら無制限）
	if limit := envInt("OPENAI_FALLBACK_MAX_REQUESTS_PER_MINUTE", 0); limit > 0 {
		if now.Sub(g.minuteWindow) >= time.Minute {
			g.minuteWindow = now
			g.minuteCount = 0
		}
		if g.minuteCount >= limit {
			return false, "分間リクエスト上限に到達"
		}
	}

	// 2) 日次・月次のUSD上限
	daily, monthly, err := g.totals(now)
	if err != nil {
		// 集計できないときは許可する。ここで止めるとローカル障害時に
		// DB 障害が重なった瞬間に AI 機能が完全停止するため。
		// 上限超過のリスクより、判定不能で閉じる方の影響が大きい。
		g.countMinute(now)
		return true, ""
	}
	if limit := envFloat("OPENAI_DAILY_HARD_LIMIT_USD", defaultDailyHardLimitUSD); limit > 0 && daily >= limit {
		return false, "日次コスト上限に到達"
	}
	if limit := envFloat("OPENAI_MONTHLY_HARD_LIMIT_USD", defaultMonthlyHardLimitUSD); limit > 0 && monthly >= limit {
		return false, "月次コスト上限に到達"
	}

	g.countMinute(now)
	return true, ""
}

func (g *OpenAIFallbackGuard) countMinute(now time.Time) {
	if g.minuteWindow.IsZero() {
		g.minuteWindow = now
	}
	g.minuteCount++
}

// totals は日次・月次のコスト合計を返す（costCacheTTL の間キャッシュする）。
func (g *OpenAIFallbackGuard) totals(now time.Time) (daily, monthly float64, err error) {
	if !g.cachedAt.IsZero() && now.Sub(g.cachedAt) < costCacheTTL {
		return g.cachedDaily, g.cachedMonthly, nil
	}
	if g.repo == nil {
		return 0, 0, errNoCostRepo
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	daily, err = g.repo.TotalCostSince(dayStart)
	if err != nil {
		return 0, 0, err
	}
	monthly, err = g.repo.TotalCostSince(monthStart)
	if err != nil {
		return 0, 0, err
	}
	g.cachedAt, g.cachedDaily, g.cachedMonthly = now, daily, monthly
	return daily, monthly, nil
}

// envFlagEnabled は既定 true のフラグ env を読む。
func envFlagEnabled(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "false", "0", "no":
		return false
	default:
		return true
	}
}

func envFloat(key string, def float64) float64 {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}
