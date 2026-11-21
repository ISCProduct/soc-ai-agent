// Package logsafe は、ログに個人情報を出さないための小さな部品。
// 学生の入力そのもの・メールアドレスをログに出さず、長さや伏せた形で出す。
package logsafe

import (
	"strings"
	"unicode/utf8"
)

// MaskEmail はログ向けにメールのローカル部を伏せる（「taro@example.com」→「t***@example.com」）。
func MaskEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.Index(email, "@")
	if at <= 0 {
		return "***"
	}
	local := email[:at]
	domain := email[at:]
	first, _ := utf8.DecodeRuneInString(local)
	return string(first) + "***" + domain
}

// MaskEmails は宛先の一覧を伏せた形にする。
func MaskEmails(emails []string) []string {
	out := make([]string, len(emails))
	for i, e := range emails {
		out[i] = MaskEmail(e)
	}
	return out
}

// Chars は文字数。学生の入力をログに出す代わりに、長さだけを出すのに使う。
func Chars(s string) int { return utf8.RuneCountInString(s) }
