package logger

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode"

	"Backend/internal/services/training/masking"
)

// logMasker は、ログの出力直前に、メールアドレスと電話番号を伏字にする。
// 氏名・住所などは、規則だけでは見分けられず、ログの文では誤検出も多いので対象にしない。
// 学生の入力そのものを出さないことは、コードの書き方（logsafe）と logpolicy の検査で守る。
// 郵便番号の規則は入れない。「0123-4567」のようなID・UUID の断片を誤って伏字にしてしまうため。
var logMasker = mustLogMasker()

func mustLogMasker() *masking.Masker {
	var rules []masking.Rule
	for _, r := range masking.DefaultRules() {
		switch r.Key {
		case masking.KeyEmail:
			rules = append(rules, r)
		case masking.KeyPhone:
			// 学習データ用の規則に、ログ向けの条件を足す。ID・UUID・16進・小数の断片を電話番号と取り違えない
			base := r.Accept
			r.Accept = func(text string, start, end int) bool {
				return (base == nil || base(text, start, end)) && acceptLogPhone(text, start, end)
			}
			rules = append(rules, r)
		}
	}
	m, err := masking.New(rules, nil)
	if err != nil {
		// 規則は固定なので、ここで失敗するのは実装の誤り。テスト（TestMustLogMasker）で起動前に気づく。
		panic("logger: ログの伏字の規則を作れません: " + err.Error())
	}
	return m
}

func redactString(s string) string { return logMasker.Mask(s) }

// acceptLogPhone は、ログの中の電話番号らしい並びを、電話番号として採るかを決める。
// ログには、UUID・sha・16進のオフセット・小数・パスなど、数字が並ぶ値が多い。
//   - 直前が英数字・「.」「-」「_」「/」（UUID の断片、ID、パス、小数の続き）なら採らない。
//   - 直後が英数字・「_」（16進の続き）なら採らない。
//   - 「0.123456789」「0-1-2-…」のように、先頭の 0 のすぐあとが区切りなら採らない（電話番号の市外局番は 2〜5 桁）。
func acceptLogPhone(text string, start, end int) bool {
	if start > 0 {
		r := lastRune(text[:start])
		if isASCIIAlnum(r) || strings.ContainsRune(".-_/", r) {
			return false
		}
	}
	if end < len(text) {
		r := firstRune(text[end:])
		if isASCIIAlnum(r) || r == '_' {
			return false
		}
	}
	if m := text[start:end]; len(m) > 1 && strings.ContainsRune(".．-‐‑–—−－ー", []rune(m[1:])[0]) {
		return false
	}
	return true
}

func isASCIIAlnum(r rune) bool {
	return r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r))
}

func lastRune(s string) rune {
	r := []rune(s)
	return r[len(r)-1]
}

func firstRune(s string) rune { return []rune(s)[0] }

// safeString は fn を呼ぶ。nil ポインタの String() などで panic したときは、ログの出力を止めず ok=false を返す。
func safeString(fn func() string) (s string, ok bool) {
	defer func() {
		if recover() != nil {
			s, ok = "", false
		}
	}()
	return fn(), true
}

// redactHandler は、メッセージと文字列の属性を伏字にしてから、元のハンドラへ渡す。
// slog.SetDefault した後は標準の log.Printf も slog を通るので、既存の呼び出しもすべて覆われる。
//
// 見る範囲: メッセージ、文字列の値、error と fmt.Stringer の値、グループの中。
// 構造体・マップなどの値は、そのまま渡す（中身は見ない）。String() を持つ値は文字列にして出す（JSON でも文字列になる）。
// String() / Error() が panic する値（nil ポインタなど）は、元の値のまま渡す。
type redactHandler struct{ inner slog.Handler }

func (h redactHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h redactHandler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, redactString(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, nr)
}

func (h redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = redactAttr(a)
	}
	return redactHandler{inner: h.inner.WithAttrs(out)}
}

func (h redactHandler) WithGroup(name string) slog.Handler {
	return redactHandler{inner: h.inner.WithGroup(name)}
}

func redactAttr(a slog.Attr) slog.Attr {
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, redactString(v.String()))
	case slog.KindGroup:
		group := v.Group()
		out := make([]slog.Attr, len(group))
		for i, g := range group {
			out[i] = redactAttr(g)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
	case slog.KindAny:
		switch x := v.Any().(type) {
		case error:
			if s, ok := safeString(x.Error); ok {
				return slog.String(a.Key, redactString(s))
			}
		case fmt.Stringer:
			if s, ok := safeString(x.String); ok {
				return slog.String(a.Key, redactString(s))
			}
		}
	}
	return a
}
