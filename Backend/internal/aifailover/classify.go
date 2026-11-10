// Package aifailover は OpenAI の障害時に、音声面接の STT / LLM / TTS をローカルの推論先へ
// 工程ごとに切り替える（#1833、親 #1831 の子B）。
//
// 既存の openai/fallback.go（ローカル → OpenAI）とは逆向きで、別の仕組みとして置く。
// 縮退するのは一時的な障害（429・5xx・タイムアウト・接続失敗）だけで、認証・入力不正のような
// 失敗はローカルで隠さずに返す。設定の誤りやリクエストの不備を、障害と取り違えないため。
//
// ローカルの推論先（Whisper.cpp / Ollama / VOICEVOX）は子A（#1832）で用意する。接続先が
// 未設定の工程は、今まで通り OpenAI だけを使う。
package aifailover

import (
	"context"
	"errors"
	"net"
	"net/http"

	"Backend/internal/openai"
)

// Degradable は、ローカルへ縮退してよい失敗かを返す。
//
// 縮退する: 429・5xx、タイムアウト、接続できない（名前解決・接続拒否など）
// 縮退しない: 認証・権限（401/403）、入力不正（400/404/413/422 など）、呼び出し元の ctx が
// 終わった（クライアントが切断した。ローカルで続けても受け取る相手がいない）
func Degradable(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	var statusErr *openai.StatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == http.StatusTooManyRequests || statusErr.StatusCode >= 500
	}
	if errors.Is(err, context.DeadlineExceeded) {
		// 呼び出し元の ctx は生きているので、内側のタイムアウト（API の応答待ち）
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}
