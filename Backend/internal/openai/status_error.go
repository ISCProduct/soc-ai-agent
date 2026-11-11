package openai

import "fmt"

// StatusError は API が 2xx 以外を返したことを表す（#1833）。
//
// 呼び出し側（OpenAI 障害時のフォールバック）が、429・5xx のような一時的な障害と、
// 認証・入力不正のような隠してはいけない失敗を状態コードで分けられるようにする。
// 文面は以前の fmt.Errorf と同じにしてあり、ログの見え方は変わらない。
type StatusError struct {
	// Format は以前のエラー文の書式（%d に状態コード、%s に本文が入る）。
	Format     string
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf(e.Format, e.StatusCode)
	}
	return fmt.Sprintf(e.Format, e.StatusCode, e.Body)
}
