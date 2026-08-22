package shared

import "errors"

// ErrForbidden は複数のサービスクラスタ・コントローラで共有される汎用の権限エラー。
var ErrForbidden = errors.New("forbidden")

// ErrNotFound は複数のサービスクラスタ・コントローラで共有される汎用のnot-foundエラー。
var ErrNotFound = errors.New("not found")

// ErrSessionFinished は終了済みの面接セッションに対する操作（Turn等）が試みられたことを表す。
// controller で 409 Conflict を返すために使用する（#1019）。
var ErrSessionFinished = errors.New("session already finished")

// ValidationError はユーザー入力起因のエラーを表す。controller で 422 を返すために使用する。
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }
