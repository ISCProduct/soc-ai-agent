package config

import (
	"os"
	"strconv"
	"strings"
)

// 複数の層で読まれる環境変数の取得口。呼び出し時に毎回読むので t.Setenv で差し替えられる。

// IsProduction は APP_ENV が production のとき true。
func IsProduction() bool { return os.Getenv("APP_ENV") == "production" }

// RAGReviewURL は RAG サービスのベース URL(前後の空白を除く)。未設定なら空文字。
func RAGReviewURL() string { return strings.TrimSpace(os.Getenv("RAG_REVIEW_URL")) }

// 秘密鍵は前後の空白を除かず、そのまま返す。検証側(LoadConfig → middleware)は os.Getenv の
// 値をそのまま使うため、ここだけ除くと末尾に改行がある環境で発行したトークンが検証で全部落ちる。
// TOKEN_ENCRYPTION_KEY も、除くと既に暗号化して保存したトークンを復号できなくなる。
func UserSecret() string         { return os.Getenv("USER_SECRET") }
func AdminSecret() string        { return os.Getenv("ADMIN_SECRET") }
func TokenEncryptionKey() string { return os.Getenv("TOKEN_ENCRYPTION_KEY") }

// FrontendURL はメール内リンクの組み立てにだけ使うので、前後の空白を除く。
func FrontendURL() string { return strings.TrimSpace(os.Getenv("FRONTEND_URL")) }

// firstEnv は最初に空でない(空白除去後の)値を返す。すべて空なら空文字。
func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// boolEnv は ParseBool できる値ならそれを、未設定・不正値なら def を返す。
func boolEnv(key string, def bool) bool {
	if b, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key))); err == nil {
		return b
	}
	return def
}

// --- 費用アラートの通知先 ---

func RealtimeAlertSlackWebhookURL() string { return firstEnv("REALTIME_ALERT_SLACK_WEBHOOK_URL") }

// OpenAICostAlertSlackWebhookURL は OpenAI 費用アラートの Slack 通知先。未設定なら Realtime 用に倒す。
func OpenAICostAlertSlackWebhookURL() string {
	return firstEnv("OPENAI_COST_ALERT_SLACK_WEBHOOK_URL", "REALTIME_ALERT_SLACK_WEBHOOK_URL")
}

func OpenAICostAlertDiscordWebhookURL() string {
	return firstEnv("OPENAI_COST_ALERT_DISCORD_WEBHOOK_URL")
}

func CompanySearchAlertSlackWebhookURL() string {
	return firstEnv("COMPANY_SEARCH_ALERT_SLACK_WEBHOOK_URL")
}

func CompanySearchAlertDiscordWebhookURL() string {
	return firstEnv("COMPANY_SEARCH_ALERT_DISCORD_WEBHOOK_URL")
}

// --- 月次上限の強制 ---

// CompanySearchBudgetEnforce は企業検索の月次上限を強制するか(既定 true)。
func CompanySearchBudgetEnforce() bool { return boolEnv("COMPANY_SEARCH_BUDGET_ENFORCE", true) }

// InterviewBudgetEnforce は面接の月次上限を強制するか(既定 false = 監視のみ)。
func InterviewBudgetEnforce() bool { return boolEnv("INTERVIEW_BUDGET_ENFORCE", false) }
