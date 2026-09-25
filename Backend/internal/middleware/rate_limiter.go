package middleware

// レート制限ミドルウェア（Issue #325 / #617）
// 既定はインメモリ。REDIS_URL 利用時は Redis スライディングウィンドウに切替可能。

import (
	"crypto/subtle"
	"net"
	"net/http"
	"os"
	"strconv"
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

// ClientIPHeader は BFF(Next.js Route Handler)が実クライアントIPを載せるヘッダー名 (#1407)。
const ClientIPHeader = "X-Client-IP"

// InternalTokenHeader は内部サービス間の共有シークレットを載せるヘッダー名。
// Backend -> RAG で既に使っている名前に揃える(Sentry のヘッダー除去対象にも入っている)。
const InternalTokenHeader = "X-Internal-Token"

// BFFInternalTokenEnv は BFF からの呼び出しであることを示す共有シークレットの環境変数名。
// frontend タスクと backend タスクへ同じ値を配る。
const BFFInternalTokenEnv = "BFF_INTERNAL_TOKEN"

// trustedForwardedClientIP は BFF が転送した実クライアントIPを返す。信用できない場合は空文字列。
//
// 信頼境界:
// backend の ALB はインターネットに直結しているため、X-Client-IP は誰でも送れる。
// 無条件に採用するとIP単位のレート制限を詐称で回避できるので、BFF と共有する
// BFF_INTERNAL_TOKEN が一致したときに限り採用する。未設定・不一致・IPとして不正な値は
// すべて無視し、呼び出し元は従来どおり ALB が付けた XFF 末尾へフォールバックする
// (シークレット未配布でもサービスが止まらないようにするため、拒否ではなく無視)。
func trustedForwardedClientIP(r *http.Request) string {
	expected := strings.TrimSpace(os.Getenv(BFFInternalTokenEnv))
	if expected == "" {
		return ""
	}
	provided := strings.TrimSpace(r.Header.Get(InternalTokenHeader))
	// 比較時間から推測されないよう定数時間比較（他の内部認証と同じ方針）
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		return ""
	}
	// BFF は1件しか載せない想定だが、経路が増えても XFF と同じ「末尾が実IP」の
	// 規則で読めるようにしておく。
	parts := strings.Split(r.Header.Get(ClientIPHeader), ",")
	ip := stripPort(strings.TrimSpace(parts[len(parts)-1]))
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

// GetClientIP は X-Forwarded-For / X-Real-IP を優先してクライアントIPを取得する。
//
// X-Forwarded-For はクライアントが自由に詐称でき、ALB は「既存の値の末尾」へ実IPを追記する。
// したがって信頼できるのは最後の要素のみ。以前は複数要素のときにカンマ区切り文字列を
// そのまま返しており、攻撃者が先頭を書き換えるだけで無限に異なるキーを作れたため、
// IP単位のレート制限(ログイン試行を含む)を完全に回避できた。
//
// BFF 経由のリクエストは XFF 末尾が frontend タスクの出口IP1つに収束し、IP単位の制限が
// 全利用者共通の上限として効いてしまう(#1407)。そのため信頼できる経路から届いた
// X-Client-IP のみを最優先で採用する。
func GetClientIP(r *http.Request) string {
	if ip := trustedForwardedClientIP(r); ip != "" {
		return ip
	}
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

// GuestAIRateLimiter は未認証で叩けるAI呼び出し（ES添削・企業WEB検索）のIP単位制限（#1154）。
// 単一送信元からの連打を止めるためのもので、利用者ごとの公平な配分ではない。
// 正規利用の大半は Next.js の BFF 経由（BACKEND_URL=https://api.shukatsu-ai.jp）で届くため
// ALB から見た送信元は frontend タスクの出口IP1つに収束する。展示会会場のNATでも同様。
// したがってIP単位は緩く取り、課金の総量は下の全体上限で止める。
var GuestAIRateLimiter KeyRateLimiter = NewRateLimiter(10*time.Minute, envPositiveInt("GUEST_AI_RATE_LIMIT_PER_IP", 300))

// GuestAIGlobalRateLimiter は同エンドポイント群の全体上限（#1154）。
// IPを変えれば per-IP 制限は回避できるため、実質的なコスト上限はこちらが担う。
var GuestAIGlobalRateLimiter KeyRateLimiter = NewRateLimiter(time.Hour, envPositiveInt("GUEST_AI_RATE_LIMIT_GLOBAL", 1000))

// envPositiveInt は環境変数を正の整数として読む。未設定・不正値は既定値。
// 展示会中に上限へ当たった場合、再ビルドせずタスク定義の環境変数だけで緩められるようにするための調整つまみ。
func envPositiveInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}
	return def
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
