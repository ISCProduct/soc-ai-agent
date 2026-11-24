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

// 秘密鍵は前後の空白を除かず、そのまま返す。検証側(LoadConfig → middleware)は os.Getenv の
// 値をそのまま使うため、ここだけ除くと末尾に改行がある環境で発行したトークンが検証で全部落ちる。
// TOKEN_ENCRYPTION_KEY も、除くと既に暗号化して保存したトークンを復号できなくなる。
func UserSecret() string         { return os.Getenv("USER_SECRET") }
func AdminSecret() string        { return os.Getenv("ADMIN_SECRET") }
func TokenEncryptionKey() string { return os.Getenv("TOKEN_ENCRYPTION_KEY") }

// FrontendURL はメール内リンクの組み立てにだけ使うので、前後の空白を除く。
func FrontendURL() string { return strings.TrimSpace(os.Getenv("FRONTEND_URL")) }
