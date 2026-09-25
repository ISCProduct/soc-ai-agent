// Package observability はエラートラッキング等の運用計装を提供する（#619 / #1185）。
package observability

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

// InitSentry は SENTRY_DSN が設定されているときだけ Sentry を初期化する。
// DSN 未設定時は no-op（ローカル開発で依存しない）。
// 戻り値の flush はプロセス終了前に呼ぶこと。
func InitSentry() (flush func(), enabled bool) {
	dsn := strings.TrimSpace(os.Getenv("SENTRY_DSN"))
	if dsn == "" {
		return func() {}, false
	}

	env := strings.TrimSpace(os.Getenv("APP_ENV"))
	if env == "" {
		env = "development"
	}
	release := strings.TrimSpace(os.Getenv("SENTRY_RELEASE"))

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      env,
		Release:          release,
		SendDefaultPII:   false,
		BeforeSend:       scrubEvent,
		TracesSampleRate: 0, // トレースは別Issue。エラー通知のみ。
	})
	if err != nil {
		slog.Error("sentry init failed", "error", err)
		return func() {}, false
	}
	slog.Info("sentry enabled", "environment", env, "release", release)
	return func() {
		sentry.Flush(2 * time.Second)
	}, true
}

// sensitiveHeaders は Sentry へ送ってはいけないリクエストヘッダー（小文字）。
// frontend の `frontend/lib/sentry.ts` の SENSITIVE_HEADERS と同じ集合にすること。
// 片方だけ直して他方を忘れる事故が実際に起きたため、TestSensitiveHeaders_フロントと同期
// が両者の突き合わせで落ちる。
var sensitiveHeaders = map[string]bool{
	"authorization":        true,
	"cookie":               true,
	"x-admin-token":        true,
	"x-user-token":         true,
	"x-company-user-token": true,
	"x-internal-token":     true,
	// CloudFront が本番の全オリジンリクエストへ付ける経路証明トークン(#1407)。
	// distribution の aliases は `*.shukatsu-ai.jp` を含むため、api ドメインを
	// CloudFront 経由で叩けば backend にもこのヘッダー付きで届く。漏れると
	// 公開ALBへの frontend 直叩きで詐称 X-Forwarded-For を署名させられる。
	"x-origin-token": true,
	// 遷移元のクエリ（＝ワンタイムトークン）がそのまま載る。
	"referer": true,
}

// scrubEvent は Sentry 送信前に認証ヘッダー・Cookie・リクエストボディを落とす。
// 履歴書・チャット本文などの個人情報が混入しないようにする（#619）。
func scrubEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event == nil {
		return nil
	}
	if event.Request != nil {
		event.Request.Cookies = ""
		event.Request.Data = ""
		for key := range event.Request.Headers {
			if sensitiveHeaders[strings.ToLower(key)] {
				delete(event.Request.Headers, key)
			}
		}
		event.Request.QueryString = ""
	}

	// 例外メッセージは素通りする。err.Error() に外部レスポンス本文や AI 出力を
	// そのまま埋めている箇所が実在する（resume_review.go の "rag review failed: %s" など）。
	// 長い本文ほど個人情報を含みやすいので、頭だけ残して切る。
	for i := range event.Exception {
		event.Exception[i].Value = truncateForSentry(event.Exception[i].Value)
	}
	event.Message = truncateForSentry(event.Message)
	return event
}

// maxSentryMessageRunes は例外メッセージとして送る上限。
// 原因の特定には先頭で足り、それ以上は本文の持ち出しになりやすい。
const maxSentryMessageRunes = 300

func truncateForSentry(msg string) string {
	runes := []rune(msg)
	if len(runes) <= maxSentryMessageRunes {
		return msg
	}
	return string(runes[:maxSentryMessageRunes]) + "…(truncated)"
}
