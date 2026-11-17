package config

import (
	"os"
	"strings"
)

// 複数の層で読まれる環境変数の取得口。呼び出し時に毎回読むので t.Setenv で差し替えられる。

// IsProduction は APP_ENV が production のとき true。
func IsProduction() bool { return os.Getenv("APP_ENV") == "production" }

// RAGReviewURL は RAG サービスのベース URL(前後の空白を除く)。未設定なら空文字。
func RAGReviewURL() string { return strings.TrimSpace(os.Getenv("RAG_REVIEW_URL")) }

func UserSecret() string         { return strings.TrimSpace(os.Getenv("USER_SECRET")) }
func AdminSecret() string        { return strings.TrimSpace(os.Getenv("ADMIN_SECRET")) }
func TokenEncryptionKey() string { return strings.TrimSpace(os.Getenv("TOKEN_ENCRYPTION_KEY")) }
func FrontendURL() string        { return strings.TrimSpace(os.Getenv("FRONTEND_URL")) }
