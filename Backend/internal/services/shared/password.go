package shared

import (
	"errors"
	"unicode/utf8"
)

const (
	// PasswordMinChars はパスワードの最小文字数。
	PasswordMinChars = 8
	// PasswordMaxBytes はパスワードの最大バイト数。bcrypt は72バイトを超える入力を受け付けない。
	PasswordMaxBytes = 72
)

var (
	// ErrPasswordTooShort はパスワードが短すぎることを表す。文言は既存の呼び出し側の判定と互換。
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	// ErrPasswordTooLong はパスワードが72バイトを超えることを表す（bcrypt の内部エラー文を利用者に出さないため）。
	ErrPasswordTooLong = errors.New("password must be at most 72 bytes")
)

// ValidatePassword はパスワードの長さを検証する。学生・企業の登録・再設定・招待受諾で共通に使う (SOCAIAGENT-693)。
func ValidatePassword(pw string) error {
	if utf8.RuneCountInString(pw) < PasswordMinChars {
		return ErrPasswordTooShort
	}
	if len(pw) > PasswordMaxBytes {
		return ErrPasswordTooLong
	}
	return nil
}
