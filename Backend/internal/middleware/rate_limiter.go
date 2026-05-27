package middleware

// インメモリ スライディングウィンドウ方式のレート制限ミドルウェア（Issue #325）
// 外部依存なし（sync.Map + time）で実装。
// キー単位（IP:email など）で window 内の試行回数を管理する。

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// rateLimitEntry は1つのキーに対するリクエスト履歴を保持する
type rateLimitEntry struct {
	mu        sync.Mutex
	timestamps []time.Time
}

// RateLimiter はスライディングウィンドウ方式のレート制限器
type RateLimiter struct {
	entries  sync.Map
	window   time.Duration
	maxReqs  int
	// クリーンアップ間隔（ゴルーティンで定期実行）
}

// NewRateLimiter は新しい RateLimiter を生成する
// window: 計測ウィンドウ幅, maxReqs: window 内の最大リクエスト数
func NewRateLimiter(window time.Duration, maxReqs int) *RateLimiter {
	rl := &RateLimiter{window: window, maxReqs: maxReqs}
	go rl.cleanupLoop()
	return rl
}

// Allow はキーに対して1リクエストを記録し、制限内なら true を返す
func (rl *RateLimiter) Allow(key string) bool {
	val, _ := rl.entries.LoadOrStore(key, &rateLimitEntry{})
	entry := val.(*rateLimitEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// 期限切れタイムスタンプを除去
	valid := entry.timestamps[:0]
	for _, t := range entry.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	entry.timestamps = valid

	if len(entry.timestamps) >= rl.maxReqs {
		return false
	}
	entry.timestamps = append(entry.timestamps, now)
	return true
}

// cleanupLoop は期限切れエントリを定期削除してメモリリークを防ぐ
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-rl.window)
		rl.entries.Range(func(k, v any) bool {
			entry := v.(*rateLimitEntry)
			entry.mu.Lock()
			valid := entry.timestamps[:0]
			for _, t := range entry.timestamps {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			entry.timestamps = valid
			empty := len(entry.timestamps) == 0
			entry.mu.Unlock()
			if empty {
				rl.entries.Delete(k)
			}
			return true
		})
	}
}

// GetClientIP は X-Forwarded-For / X-Real-IP を優先してクライアントIPを取得する
func GetClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip, _, err := net.SplitHostPort(xff); err == nil {
			return ip
		}
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// LoginRateLimiter はログイン試行のレート制限器
// IP単位: 1分間に20回まで（正常ユーザーの誤入力を許容しつつ攻撃を防ぐ）
var LoginRateLimiter = NewRateLimiter(time.Minute, 20)

// PasswordResetRateLimiter はパスワードリセット要求のレート制限器
// IP単位: 1時間に5回まで（メールサーバー保護）
var PasswordResetRateLimiter = NewRateLimiter(time.Hour, 5)

// LoginRateLimit はログインエンドポイントのレート制限ミドルウェア
func LoginRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r)
		if !LoginRateLimiter.Allow(ip) {
			http.Error(w, "Too Many Requests: お試し回数の上限に達しました。しばらく待ってから再試行してください。", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// PasswordResetRateLimit はパスワードリセットエンドポイントのレート制限ミドルウェア
func PasswordResetRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r)
		if !PasswordResetRateLimiter.Allow(ip) {
			http.Error(w, "Too Many Requests: リクエスト上限に達しました。しばらく待ってから再試行してください。", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
