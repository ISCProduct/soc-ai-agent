package costs

import (
	"errors"
	"testing"
	"time"
)

// fakeCostRepo は日次/月次の集計を差し替えるテスト用リポジトリ。
type fakeCostRepo struct {
	// since ごとの返り値。日次(その日の0時)と月次(1日の0時)で引き分ける。
	daily, monthly float64
	err            error
	calls          int
}

func (r *fakeCostRepo) TotalCostSince(since time.Time) (float64, error) {
	r.calls++
	if r.err != nil {
		return 0, r.err
	}
	if since.Day() == 1 && since.Hour() == 0 {
		// 月初は日次と月次が同じ範囲になるため、テストでは月次を優先して返す
		return r.monthly, nil
	}
	if since.Day() == since.Day() && since.Hour() == 0 && since.Month() != 0 {
		// 日次の問い合わせ（その日の0時）と月次（1日の0時）を区別する
		if since.Day() == 1 {
			return r.monthly, nil
		}
		return r.daily, nil
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
	if repo.calls != 2 {
		t.Fatalf("集計クエリ数=%d want 2（日次・月次を1回ずつ）", repo.calls)
	}

	// TTL 経過後は再取得する
	g.now = func() time.Time { return at.Add(costCacheTTL + time.Second) }
	if allow, _ := g.AllowFallback(); !allow {
		t.Fatal("上限内なのに拒否された")
	}
	if repo.calls != 4 {
		t.Fatalf("TTL経過後の集計クエリ数=%d want 4", repo.calls)
	}
}

// TestAllowFallback_NilGuard は未設定のガードが拒否側に倒れることを検証する。
func TestAllowFallback_NilGuard(t *testing.T) {
	var g *OpenAIFallbackGuard
	if allow, _ := g.AllowFallback(); allow {
		t.Fatal("nil ガードが許可を返した")
	}
}
