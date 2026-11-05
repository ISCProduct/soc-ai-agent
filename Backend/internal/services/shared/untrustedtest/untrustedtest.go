// Package untrustedtest は shared.WrapUntrustedText の囲みをテストで確かめる補助。
package untrustedtest

import (
	"regexp"
	"strings"
	"testing"
)

// AssertInside は needle が label の囲みの内側にだけあることを確かめる。
// 囲みを外すと（外側に出る／ブロックが無い）落ちる。
func AssertInside(t *testing.T, prompt, label, needle string) {
	t.Helper()
	re := regexp.MustCompile(`<<<(UNTRUSTED_` + regexp.QuoteMeta(strings.ToUpper(label)) + `_[A-Z2-7]+)_START>>>\n`)
	m := re.FindStringSubmatchIndex(prompt)
	if m == nil {
		t.Fatalf("%s が囲まれていない:\n%s", label, prompt)
	}
	marker := prompt[m[2]:m[3]]
	// 宣言文にも同じ区切りが出るので、ブロック開始より後ろの終了区切りを探す。
	rel := strings.Index(prompt[m[1]:], "<<<"+marker+"_END>>>")
	if rel < 0 {
		t.Fatalf("%s の終了区切りが無い", label)
	}
	end := m[1] + rel
	if !strings.Contains(prompt[m[1]:end], needle) {
		t.Fatalf("%q が %s の内側に無い:\n%s", needle, label, prompt)
	}
	if strings.Contains(prompt[:m[0]]+prompt[end:], needle) {
		t.Fatalf("%q が %s の外側にも出ている:\n%s", needle, label, prompt)
	}
}
