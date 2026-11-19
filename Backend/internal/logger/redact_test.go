package logger

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"strings"
	"testing"
)

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(redactHandler{inner: slog.NewTextHandler(buf, &slog.HandlerOptions{})})
}

// メール・電話番号は、メッセージにも、文字列・error・グループ・WithAttrs の属性にも残らない。
func TestRedactHandler_MasksEmailAndPhone(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf).With("fixed", "連絡先 hanako@example.org")
	l.Info("send to taro@example.com", "tel", "090-1234-5678", "err", errors.New("failed for taro@example.com"),
		slog.Group("g", slog.String("mail", "a.b@example.jp")))
	out := buf.String()
	for _, leak := range []string{"taro@example.com", "090-1234-5678", "hanako@example.org", "a.b@example.jp"} {
		if strings.Contains(out, leak) {
			t.Errorf("%q が残った: %s", leak, out)
		}
	}
	if !strings.Contains(out, "<EMAIL>") || !strings.Contains(out, "<PHONE>") {
		t.Errorf("記号に置き換わっていない: %s", out)
	}
}

// 標準の log.Printf も（slog.SetDefault 後は）同じハンドラを通る。
func TestRedactHandler_CoversStdLog(t *testing.T) {
	var buf bytes.Buffer
	prevLogger, prevFlags, prevOut := slog.Default(), log.Flags(), log.Writer()
	t.Cleanup(func() { slog.SetDefault(prevLogger); log.SetFlags(prevFlags); log.SetOutput(prevOut) })
	slog.SetDefault(newTestLogger(&buf))
	log.Printf("[Auth] resend for taro@example.com tel=09012345678")
	if out := buf.String(); strings.Contains(out, "taro@example.com") || strings.Contains(out, "09012345678") {
		t.Errorf("標準の log の出力に残った: %s", out)
	}
}

// 普通のログ（ID・時刻・UUID・件数・処理時間）は変えない。誤って伏字にすると、障害の調査ができなくなる。
func TestRedactHandler_KeepsOrdinaryLogs(t *testing.T) {
	for _, in := range []string{
		"[openai] model=gpt-5 status=200 latency=1234ms",
		"session=12345 turn=3 attempt=2/3",
		"request_id=f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"request_id=0b7e1d2c-0123-4567-8901-0e02b2c3d479",
		"2026-11-18T07:28:14Z user_id=42 bytes=16000",
		"score=0.123 hit=75.00% cached=1024/2048",
		"time=1763450894123456789 elapsed=0.0123s",
	} {
		if got := redactString(in); got != in {
			t.Errorf("普通のログが変わった: %q → %q", in, got)
		}
	}
}

// UUID・sha・16進を乱数で大量に流しても、変わらないこと。固定の数例だけでは、約1%の誤検出を見逃す。
func TestRedactHandler_KeepsRandomIDs(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	hex := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = "0123456789abcdef"[rnd.Intn(16)]
		}
		return string(b)
	}
	for i := 0; i < 5000; i++ {
		for _, in := range []string{
			fmt.Sprintf("request_id=%s-%s-4%s-8%s-%s", hex(8), hex(4), hex(3), hex(3), hex(12)),
			"sha=" + hex(64),
			"key=" + hex(20),
		} {
			if got := redactString(in); got != in {
				t.Fatalf("ID を伏字にした: %q → %q", in, got)
			}
		}
	}
}

// 小数・パス・16進のオフセットなど、数字が並ぶ値も変えない。電話番号は、区切りの違いや前後の文字があっても伏字にする。
func TestRedactHandler_PhoneBoundaries(t *testing.T) {
	for _, in := range []string{
		"score=0.123456789", "x=0.1234567890", "/var/log/0123456789.log", "id=A0123456789", "offset=0x0123456789", "v=0.5.0.1234567", "0-1-2-3-4-5-6-7-8-9",
	} {
		if got := redactString(in); got != in {
			t.Errorf("電話番号ではない値を伏字にした: %q → %q", in, got)
		}
	}
	for _, in := range []string{
		"tel=09012345678", "TEL:090-1234-5678", "連絡は090-1234-5678です", "(090)1234-5678", "tel 03-1234-5678.", "携帯：０９０－１２３４－５６７８", "+81 90-1234-5678",
	} {
		if got := redactString(in); !strings.Contains(got, "<PHONE>") {
			t.Errorf("電話番号が残った: %q → %q", in, got)
		}
	}
}

type nilUnsafeStringer struct{ n int }

func (s *nilUnsafeStringer) String() string { return fmt.Sprint(s.n) }

// String() が panic する値（nil ポインタ）で、ログの呼び出しが落ちないこと。
func TestRedactHandler_NilStringerDoesNotPanic(t *testing.T) {
	var buf bytes.Buffer
	var p *nilUnsafeStringer
	newTestLogger(&buf).Info("x", "p", p, "ok", &nilUnsafeStringer{n: 7})
	if !strings.Contains(buf.String(), "ok=7") {
		t.Errorf("通常の Stringer が文字列になっていない: %s", buf.String())
	}
}
