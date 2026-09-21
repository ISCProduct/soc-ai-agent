// Package safego は panic でプロセスごと落ちない goroutine 起動を提供する。
package safego

import (
	"log/slog"
	"runtime/debug"

	"github.com/getsentry/sentry-go"
)

// Go は fn を goroutine で実行し、panic を捕まえてログと Sentry に残す。
//
// Go では goroutine 内の panic は誰も回復しないとプロセス全体を落とす。
// Echo の Recover ミドルウェアが守るのはリクエストを処理している goroutine だけで、
// そこから `go` で切り離した処理（メール送信・S3 アップロード・再マッチング・
// 利用量記録など）が1つ落ちると Backend が丸ごと死に、全ユーザーが 502 になる。
//
// 投げっぱなしの処理専用。呼び出し元へ結果を返す goroutine には使わない
// （panic を握り潰すと受け側がチャネル待ちで止まるため、その場で defer recover して
// エラーを送り返すこと）。
func Go(fn func()) {
	go func() {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			slog.Error("goroutine が panic しました", "panic", r, "stack", string(debug.Stack()))
			// Sentry 未初期化なら CurrentHub にクライアントが無く、何もしない。
			sentry.CurrentHub().Recover(r)
		}()
		fn()
	}()
}
