package shared

import "errors"

// ErrForbidden は複数のサービスクラスタ・コントローラで共有される汎用の権限エラー。
var ErrForbidden = errors.New("forbidden")

// ValidationError はユーザー入力起因のエラーを表す。controller で 422 を返すために使用する。
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }
