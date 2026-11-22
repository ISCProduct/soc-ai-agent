package shared

import (
	"errors"
	"strings"
	"testing"
)

// パスワードの長さは「8文字以上（文字数）・72バイト以下（bcrypt の上限）」で、学生・企業の全経路で共通 (SOCAIAGENT-693)。
func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name string
		pw   string
		want error
	}{
		{name: "空", pw: "", want: ErrPasswordTooShort},
		{name: "7文字", pw: "abcdefg", want: ErrPasswordTooShort},
		{name: "8文字", pw: "abcdefgh", want: nil},
		{name: "日本語8文字（24バイト）", pw: strings.Repeat("あ", 8), want: nil},
		{name: "日本語7文字（21バイト）は文字数で不足", pw: strings.Repeat("あ", 7), want: ErrPasswordTooShort},
		{name: "72バイト", pw: strings.Repeat("a", 72), want: nil},
		{name: "73バイト", pw: strings.Repeat("a", 73), want: ErrPasswordTooLong},
		{name: "日本語25文字（75バイト）", pw: strings.Repeat("あ", 25), want: ErrPasswordTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidatePassword(tt.pw); !errors.Is(got, tt.want) {
				t.Errorf("ValidatePassword() = %v, want %v", got, tt.want)
			}
		})
	}
}
