// Package ragclient は RAG サービス呼び出しの共通処理を提供する (#615)
package ragclient

import (
	"net/http"
	"os"
	"strings"

	"Backend/internal/middleware"
)

// InternalTokenHeader は RAG サービス内部認証用のヘッダー名
const InternalTokenHeader = "X-Internal-Token"

const (
	// TraceIDHeader は RAG 側が読むヘッダー名（rag/main.py の _trace_id_middleware）
	TraceIDHeader = "X-Trace-ID"
	// RequestIDHeader は Backend 側のヘッダー名。RAG も予備として読む
	RequestIDHeader = "X-Request-ID"
)

// SetAuthHeader は RAG_INTERNAL_TOKEN が設定されていれば内部認証ヘッダーを付与し、
// あわせてリクエストIDを RAG へ伝播する（#1188）。
//
// Backend は X-Request-ID を受け取る/生成してログに出し、RAG は X-Trace-ID を
// 受け取る/生成してログに出していたが、境界で渡していなかったため両者のログを
// 突き合わせられなかった。ここが RAG 呼び出しの唯一の共通経路なので、
// ヘッダー付与もここに集約する。
//
// リクエストIDはコンテキストから取る。ctx を持たない http.NewRequest で作られた
// リクエストでは空になるため、呼び出し側は NewRequestWithContext を使う。
func SetAuthHeader(req *http.Request) {
	if token := strings.TrimSpace(os.Getenv("RAG_INTERNAL_TOKEN")); token != "" {
		req.Header.Set(InternalTokenHeader, token)
	}
	if rid := middleware.GetRequestID(req.Context()); rid != "" {
		req.Header.Set(TraceIDHeader, rid)
		req.Header.Set(RequestIDHeader, rid)
	}
}
