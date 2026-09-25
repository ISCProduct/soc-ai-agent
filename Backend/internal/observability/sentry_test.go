package observability

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/getsentry/sentry-go"
)

func TestInitSentry_NoDSN(t *testing.T) {
	t.Setenv("SENTRY_DSN", "")
	flush, enabled := InitSentry()
	if enabled {
		t.Fatal("DSN 未設定なのに enabled=true")
	}
	flush() // panicしないこと
}

func TestScrubEvent_RemovesSecrets(t *testing.T) {
	ev := &sentry.Event{
		Request: &sentry.Request{
			Cookies: "session=abc",
			Data:    `{"resume_text":"秘密の職務経歴"}`,
			Headers: map[string]string{
				"Authorization":        "Bearer secret",
				"Cookie":               "a=1",
				"X-Admin-Token":        "admin-secret",
				"X-User-Token":         "user-secret",
				"X-Company-User-Token": "company-secret",
				"X-Internal-Token":     "rag-secret",
				// CloudFront は aliases の `*.shukatsu-ai.jp` 経由で api ドメインにも
				// このヘッダーを付けて到達する。Sentry に残ると公開ALBへの frontend
				// 直叩きで詐称 X-Forwarded-For を署名させられる(#1407)。
				"x-origin-token": "cf-secret",
				"Referer":        "https://shukatsu-ai.jp/verify-email?token=ONETIME",
				"X-Request-ID":   "req-keep",
				"Content-Type":   "application/json",
			},
			QueryString: "email=student@example.com",
		},
	}

	out := scrubEvent(ev, nil)
	if out.Request.Cookies != "" {
		t.Errorf("Cookies = %q, want empty", out.Request.Cookies)
	}
	if out.Request.Data != "" {
		t.Fatalf("Data が残っている: %q", out.Request.Data)
	}
	if out.Request.QueryString != "" {
		t.Fatalf("QueryString が残っている: %q", out.Request.QueryString)
	}
	for _, key := range []string{
		"Authorization", "Cookie", "X-Admin-Token", "X-User-Token",
		"X-Company-User-Token", "X-Internal-Token", "x-origin-token", "Referer",
	} {
		if _, ok := out.Request.Headers[key]; ok {
			t.Errorf("%s が残っている", key)
		}
	}
	if got := out.Request.Headers["X-Request-ID"]; got != "req-keep" {
		t.Fatalf("X-Request-ID = %q, want req-keep", got)
	}
}

func TestScrubEvent_NilSafe(t *testing.T) {
	if scrubEvent(nil, nil) != nil {
		t.Fatal("nil event は nil を返すべき")
	}
}

// 例外メッセージには外部レスポンス本文や AI 出力がそのまま入る経路がある
// （resume_review.go の "rag review failed: %s" など）。Request のスクラブだけでは
// 個人情報の混入を防げない。
func TestScrubEvent_例外メッセージを切り詰める(t *testing.T) {
	long := strings.Repeat("あ", maxSentryMessageRunes+50)
	event := &sentry.Event{
		Message:   long,
		Exception: []sentry.Exception{{Value: "rag review failed: " + long}},
	}

	out := scrubEvent(event, nil)

	if utf8.RuneCountInString(out.Exception[0].Value) > maxSentryMessageRunes+len("…(truncated)") {
		t.Errorf("例外メッセージが切り詰められていない: %d文字", utf8.RuneCountInString(out.Exception[0].Value))
	}
	if !strings.HasSuffix(out.Exception[0].Value, "…(truncated)") {
		t.Error("切り詰めたことが分かる印が無い")
	}
	if utf8.RuneCountInString(out.Message) > maxSentryMessageRunes+len("…(truncated)") {
		t.Error("Message が切り詰められていない")
	}
}

func TestScrubEvent_短いメッセージはそのまま(t *testing.T) {
	event := &sentry.Event{Exception: []sentry.Exception{{Value: "db connection failed"}}}
	if got := scrubEvent(event, nil).Exception[0].Value; got != "db connection failed" {
		t.Errorf("短いメッセージまで加工している: %q", got)
	}
}

// 秘匿ヘッダーの一覧は frontend(lib/sentry.ts)と Backend の2箇所にある。
// 片方へ足して他方を忘れると、同じヘッダーが一方の Sentry にだけ残る
// （X-Origin-Token で実際に起きた）。両者を突き合わせて食い違いで落とす。
func TestSensitiveHeaders_フロントと同期(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "frontend", "lib", "sentry.ts"))
	if err != nil {
		t.Fatalf("frontend/lib/sentry.ts を読めない（移動したならこのテストも直す）: %v", err)
	}
	block := regexp.MustCompile(`(?s)SENSITIVE_HEADERS = new Set\(\[(.*?)\]\)`).FindSubmatch(src)
	if block == nil {
		t.Fatal("frontend/lib/sentry.ts の SENSITIVE_HEADERS を見つけられない")
	}

	front := map[string]bool{}
	for _, m := range regexp.MustCompile(`'([^']+)'`).FindAllSubmatch(block[1], -1) {
		front[string(m[1])] = true
	}
	if len(front) == 0 {
		t.Fatal("frontend 側のヘッダーを1つも抽出できていない（記法が変わった可能性）")
	}

	for h := range front {
		if !sensitiveHeaders[h] {
			t.Errorf("frontend にだけある: %s（Backend の sensitiveHeaders にも足す）", h)
		}
	}
	for h := range sensitiveHeaders {
		if !front[h] {
			t.Errorf("Backend にだけある: %s（frontend の SENSITIVE_HEADERS にも足す）", h)
		}
	}
}
