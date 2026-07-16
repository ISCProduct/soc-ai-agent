// Package ragclient は RAG サービス呼び出しの共通処理を提供する (#615)
package ragclient

import (
	"net/http"
	"os"
	"strings"
)

// InternalTokenHeader は RAG サービス内部認証用のヘッダー名
const InternalTokenHeader = "X-Internal-Token"

// SetAuthHeader は RAG_INTERNAL_TOKEN が設定されていれば内部認証ヘッダーを付与する。
// 未設定時はヘッダーを付けずに送信し、RAG 側のフェイルクローズ（503/401）に委ねる。
func SetAuthHeader(req *http.Request) {
	if token := strings.TrimSpace(os.Getenv("RAG_INTERNAL_TOKEN")); token != "" {
		req.Header.Set(InternalTokenHeader, token)
	}
}
