package costs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeCostRepo は日次/月次の集計を差し替えるテスト用リポジトリ。
type fakeCostRepo struct {
	// since ごとの返り値。日次(その日の0時)と月次(1日の0時)で引き分ける。
	daily, monthly float64
	err            error
	calls          atomic.Int32
}

func (r *fakeCostRepo) TotalFallbackCostSince(ctx context.Context, since time.Time) (float64, error) {
	r.calls.Add(1)
	if r.err != nil {
		return 0, r.err
	}
	// 月次の問い合わせは1日の0時。それ以外はその日の0時（日次）。
	if since.Day() == 1 {
		return r.monthly, nil
	}
	return r.daily, nil
}

func newGuardAt(repo costRepo, at time.Time) *OpenAIFallbackGuard {
	g := NewOpenAIFallbackGuard(repo)
	g.now = func() time.Time { return at }
	return g
}

// TestAllowFallback_Limits は上限判定をテーブル駆動で検証する（#1293）。
func TestAllowFallback_Limits(t *testing.T) {
	// 15日を基準にする（日次と月次の問い合わせが別の since になる）
	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		env       map[string]string
		daily     float64
		monthly   float64
		repoErr   error
		wantAllow bool
	}{
		{name: "上限内なら許可", daily: 0.5, monthly: 5, wantAllow: true},
		{name: "日次上限に到達したら拒否", daily: 2.0, monthly: 5},
		{name: "日次上限を超えたら拒否", daily: 2.5, monthly: 5},
		{name: "月次上限に到達したら拒否", daily: 0.1, monthly: 20},
		{
			name:  "env で上限を引き上げれば許可",
			env:   map[string]string{"OPENAI_DAILY_HARD_LIMIT_USD": "10", "OPENAI_MONTHLY_HARD_LIMIT_USD": "100"},
			daily: 2.5, monthly: 30, wantAllow: true,
		},
		{
			name:  "env で上限を0にすると無制限",
			env:   map[string]string{"OPENAI_DAILY_HARD_LIMIT_USD": "0", "OPENAI_MONTHLY_HARD_LIMIT_USD": "0"},
			daily: 999, monthly: 9999, wantAllow: true,
		},
		{
			name:  "不正な env 値は既定値を使う",
			env:   map[string]string{"OPENAI_DAILY_HARD_LIMIT_USD": "abc"},
			daily: 2.0, monthly: 1,
		},
		{
			name:  "OPENAI_FALLBACK_ENABLED=false なら常に拒否",
			env:   map[string]string{"OPENAI_FALLBACK_ENABLED": "false"},
			daily: 0, monthly: 0,
		},
		{
			// 集計できないときに閉じると、ローカル障害とDB障害が重なった瞬間に
			// AI 機能が完全停止する。上限超過のリスクより影響が大きいので許可する
			name: "集計に失敗したら許可する（fail-open）", repoErr: errors.New("db down"), wantAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, k := range []string{
				"OPENAI_FALLBACK_ENABLED", "OPENAI_DAILY_HARD_LIMIT_USD",
				"OPENAI_MONTHLY_HARD_LIMIT_USD", "OPENAI_FALLBACK_MAX_REQUESTS_PER_MINUTE",
			} {
				t.Setenv(k, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			repo := &fakeCostRepo{daily: tt.daily, monthly: tt.monthly, err: tt.repoErr}
			allow, reason := newGuardAt(repo, at).AllowFallback()
			if allow != tt.wantAllow {
				t.Fatalf("allow = %v (%s), want %v", allow, reason, tt.wantAllow)
			}
			if !allow && reason == "" {
				t.Error("拒否したのに理由が空（ログで気づけない）")
			}
		})
	}
}

// TestAllowFallback_MinuteRateLimit は分間リクエスト上限を検証する。
func TestAllowFallback_MinuteRateLimit(t *testing.T) {
	t.Setenv("OPENAI_FALLBACK_MAX_REQUESTS_PER_MINUTE", "2")
	t.Setenv("OPENAI_DAILY_HARD_LIMIT_USD", "")
	t.Setenv("OPENAI_MONTHLY_HARD_LIMIT_USD", "")

	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	repo := &fakeCostRepo{daily: 0, monthly: 0}
	g := newGuardAt(repo, at)

	for i := range 2 {
		if allow, reason := g.AllowFallback(); !allow {
			t.Fatalf("%d回目で拒否された: %s", i+1, reason)
		}
	}
	if allow, reason := g.AllowFallback(); allow {
		t.Fatal("上限を超えても許可された")
	} else if reason == "" {
		t.Error("理由が空")
	}

	// 1分経過すればウィンドウがリセットされる
	g.now = func() time.Time { return at.Add(61 * time.Second) }
	if allow, reason := g.AllowFallback(); !allow {
		t.Fatalf("ウィンドウ経過後も拒否された: %s", reason)
	}
}

// TestAllowFallback_CachesTotals は集計をキャッシュしてDBを叩き続けないことを検証する。
// 障害時はフォールバックが連続するため、毎回2クエリ走ると負荷になる。
func TestAllowFallback_CachesTotals(t *testing.T) {
	t.Setenv("OPENAI_FALLBACK_ENABLED", "")
	t.Setenv("OPENAI_DAILY_HARD_LIMIT_USD", "")
	t.Setenv("OPENAI_MONTHLY_HARD_LIMIT_USD", "")

	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	repo := &fakeCostRepo{daily: 0.1, monthly: 1}
	g := newGuardAt(repo, at)

	for range 5 {
		if allow, _ := g.AllowFallback(); !allow {
			t.Fatal("上限内なのに拒否された")
		}
	}
	if got := repo.calls.Load(); got != 2 {
		t.Fatalf("集計クエリ数=%d want 2（日次・月次を1回ずつ）", got)
	}

	// TTL 経過後は再取得する
	g.now = func() time.Time { return at.Add(costCacheTTL + time.Second) }
	if allow, _ := g.AllowFallback(); !allow {
		t.Fatal("上限内なのに拒否された")
	}
	if got := repo.calls.Load(); got != 4 {
		t.Fatalf("TTL経過後の集計クエリ数=%d want 4", got)
	}
}

// TestAllowFallback_NilGuard は未設定のガードが拒否側に倒れることを検証する。
func TestAllowFallback_NilGuard(t *testing.T) {
	var g *OpenAIFallbackGuard
	if allow, _ := g.AllowFallback(); allow {
		t.Fatal("nil ガードが許可を返した")
	}
}

// TestAllowFallback_MinuteLimitDefault は分間上限の既定値が無制限でないことを固定する。
//
// USD 上限は最大 costCacheTTL 分だけ古い値で判定するため、その窓の間に効く
// ブレーキはこのレート制限だけ（#1293 レビュー）。
func TestAllowFallback_MinuteLimitDefault(t *testing.T) {
	for _, k := range []string{
		"OPENAI_FALLBACK_ENABLED", "OPENAI_DAILY_HARD_LIMIT_USD",
		"OPENAI_MONTHLY_HARD_LIMIT_USD", "OPENAI_FALLBACK_MAX_REQUESTS_PER_MINUTE",
	} {
		t.Setenv(k, "")
	}
	if defaultMaxRequestsPerMinute <= 0 {
		t.Fatalf("既定の分間上限が無制限になっている: %d", defaultMaxRequestsPerMinute)
	}

	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	g := newGuardAt(&fakeCostRepo{daily: 0, monthly: 0}, at)

	for i := range defaultMaxRequestsPerMinute {
		if allow, reason := g.AllowFallback(); !allow {
			t.Fatalf("%d 回目で拒否された: %s", i+1, reason)
		}
	}
	if allow, reason := g.AllowFallback(); allow {
		t.Error("既定の分間上限を超えても許可された")
	} else if reason != "分間リクエスト上限に到達" {
		t.Errorf("reason = %q", reason)
	}
}

// TestAllowFallback_ConcurrentIsRaceFree は並行呼び出しでカウンタが壊れないことを確認する。
// 障害時は複数ハンドラから同時に呼ばれる（-race で実行する）。
func TestAllowFallback_ConcurrentIsRaceFree(t *testing.T) {
	for _, k := range []string{
		"OPENAI_FALLBACK_ENABLED", "OPENAI_DAILY_HARD_LIMIT_USD",
		"OPENAI_MONTHLY_HARD_LIMIT_USD", "OPENAI_FALLBACK_MAX_REQUESTS_PER_MINUTE",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("OPENAI_FALLBACK_MAX_REQUESTS_PER_MINUTE", "1000")

	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	g := newGuardAt(&fakeCostRepo{daily: 0, monthly: 0}, at)

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.AllowFallback()
		}()
	}
	wg.Wait()

	g.mu.Lock()
	count := g.minuteCount
	g.mu.Unlock()
	if count != 50 {
		t.Errorf("minuteCount = %d, want 50", count)
	}
}

// TestCalculateCost_NonOpenAIProviderIsFree はローカル推論を課金額に混ぜないことを検証する。
//
// 未知モデル名は gpt-4o 単価にフォールバックするため、これが無いと
// 無料のローカル推論が架空コストとして日次/月次予算を食い潰す（#1293 レビュー）。
func TestCalculateCost_NonOpenAIProviderIsFree(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
		wantZero bool
	}{
		{name: "localは常に0", provider: "local", model: "my-local-model", wantZero: true},
		{name: "localでOpenAI名でも0", provider: "local", model: "gpt-4o", wantZero: true},
		{name: "openaiは課金", provider: "openai", model: "gpt-4o"},
		{name: "provider不明(既存行)はOpenAI扱い", provider: "", model: "gpt-4o"},
		{name: "大文字小文字を区別しない", provider: "LOCAL", model: "gpt-4o", wantZero: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateCost(tt.provider, tt.model, 1_000_000, 1_000_000)
			if tt.wantZero && got != 0 {
				t.Errorf("cost = %v, want 0", got)
			}
			if !tt.wantZero && got <= 0 {
				t.Errorf("cost = %v, want > 0", got)
			}
		})
	}
}
