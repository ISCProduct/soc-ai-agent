package middleware

// レート制限ミドルウェア（Issue #325 / #617）
// 既定はインメモリ。REDIS_URL 利用時は Redis スライディングウィンドウに切替可能。

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// KeyRateLimiter はキー単位のレート制限インターフェース（#617）。
type KeyRateLimiter interface {
	Allow(key string) bool
}

// rateLimitEntry は1つのキーに対するリクエスト履歴を保持する
type rateLimitEntry struct {
	mu         sync.Mutex
	timestamps []time.Time
}

// RateLimiter はスライディングウィンドウ方式のインメモリレート制限器
type RateLimiter struct {
	entries sync.Map
	window  time.Duration
	maxReqs int
}

// NewRateLimiter は新しいインメモリ RateLimiter を生成する
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

// GetClientIP は X-Forwarded-For / X-Real-IP を優先してクライアントIPを取得する。
//
// X-Forwarded-For はクライアントが自由に詐称でき、ALB は「既存の値の末尾」へ実IPを追記する。
// したがって信頼できるのは最後の要素のみ。以前は複数要素のときにカンマ区切り文字列を
// そのまま返しており、攻撃者が先頭を書き換えるだけで無限に異なるキーを作れたため、
// IP単位のレート制限(ログイン試行を含む)を完全に回避できた。
func GetClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if last := strings.TrimSpace(parts[len(parts)-1]); last != "" {
			return stripPort(last)
		}
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return stripPort(xri)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// stripPort は "IP:port" 形式ならホスト部を返す。XFF の要素は通常IPのみだが、
// ポート付きで送出する実装もあるため正規化してキーの重複を防ぐ。
func stripPort(v string) string {
	if host, _, err := net.SplitHostPort(v); err == nil {
		return host
	}
	return v
}

// LoginRateLimiter はログイン試行のレート制限器（差し替え可能）
// IP単位: 1分間に20回まで
var LoginRateLimiter KeyRateLimiter = NewRateLimiter(time.Minute, 20)

// PasswordResetRateLimiter はパスワードリセット要求のレート制限器（差し替え可能）
// IP単位: 1時間に5回まで
var PasswordResetRateLimiter KeyRateLimiter = NewRateLimiter(time.Hour, 5)

// ConfigureRateLimiters は外部 KeyRateLimiter でグローバル制限器を差し替える（#617）。
func ConfigureRateLimiters(login, passwordReset KeyRateLimiter) {
	if login != nil {
		LoginRateLimiter = login
	}
	if passwordReset != nil {
		PasswordResetRateLimiter = passwordReset
	}
}

// CompanyEntryRateLimiter は企業情報ゲスト投稿のレート制限器（#754）
// IP単位: 1時間に5回まで
var CompanyEntryRateLimiter = NewRateLimiter(time.Hour, 5)

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
