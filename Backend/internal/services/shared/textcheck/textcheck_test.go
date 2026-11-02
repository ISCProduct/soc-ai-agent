package textcheck_test

import (
	"strings"
	"testing"

	"Backend/internal/services/shared/textcheck"
)

type ruleCase struct {
	name string
	text string
	want []string // 指摘される箇所の文字列（出現順）
}

// flagged は指摘位置の文字列を返す。位置の検証を文字列で読めるようにするため。
func flagged(text string, fs []textcheck.Finding) []string {
	rs := []rune(text)
	out := []string{}
	for _, f := range fs {
		out = append(out, string(rs[f.Start:f.End]))
	}
	return out
}

func checkCases(t *testing.T, rule textcheck.Rule, wantRule string, cases []ruleCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := rule.Check(tc.text)
			got := flagged(tc.text, fs)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") || len(got) != len(tc.want) {
				t.Fatalf("指摘箇所 = %q, want %q", got, tc.want)
			}
			for _, f := range fs {
				if f.Rule != wantRule || f.Severity == "" || f.Message == "" || f.Fix == "" {
					t.Errorf("指摘の項目が欠けている: %+v", f)
				}
			}
		})
	}
}

func TestMixedWidth(t *testing.T) {
	checkCases(t, textcheck.MixedWidth{}, "mixed_width", []ruleCase{
		{"数字は少ない側（全角）を指摘", "3年間で５人増えた。2年目に7人", []string{"５"}},
		{"全角だけなら混在ではない", "２０２４年に３人", nil},
		{"英字が同数なら全角側を指摘", "ITとＩＴ", []string{"ＩＴ"}},
		{"数字と英字は別々に判定する", "ＡＢＣで3位", nil},
	})
}

func TestMixedNumeral(t *testing.T) {
	checkCases(t, textcheck.MixedNumeral{}, "mixed_numeral", []ruleCase{
		{"同じ対象が両方の表記なら同数は漢数字側", "三年間続け、最後の3年目に", []string{"三年"}},
		{"少ない側を指摘", "3人、5人、七人", []string{"七人"}},
		{"漢数字だけなら指摘しない", "一つの目標を立てた。", nil},
		{"熟語の一部の漢数字は数えない", "第一志望で、1つ目の", nil},
		{"数える対象が違えば混在ではない", "三年間で5人", nil},
	})
}

func TestMixedEra(t *testing.T) {
	checkCases(t, textcheck.MixedEra{}, "mixed_era", []ruleCase{
		{"同数なら和暦側を指摘", "2020年4月入学、令和6年3月卒業", []string{"令和6年"}},
		{"和暦だけなら指摘しない", "令和元年と令和2年", nil},
		{"少ない側を指摘", "2019年、2020年、平成30年", []string{"平成30年"}},
	})
}

func TestMixedStyle(t *testing.T) {
	checkCases(t, textcheck.MixedStyle{}, "mixed_style", []ruleCase{
		{"少ない側（である調）の文を指摘", "私は部長です。毎日練習した。大会に出場しました。", []string{"毎日練習した。"}},
		{"敬体だけなら指摘しない", "私は部長です。大会に出場しました。", nil},
		{"体言止めは数えない", "目標は全国大会。練習しました。", nil},
		{"改行だけで終わる行は文とみなさない", "練習を重ねた\n大会に出場しました。", nil},
	})
}

func TestDateOrder(t *testing.T) {
	checkCases(t, textcheck.DateOrder{}, "date_order", []ruleCase{
		{"開始が終了より後", "2021年4月〜2020年3月 テニス部", []string{"2021年4月〜2020年3月"}},
		{"正しい順序", "2020年4月〜2021年3月", nil},
		{"和暦も西暦に直して比べる", "令和3年4月から令和2年3月", []string{"令和3年4月から令和2年3月"}},
		{"元号をまたぐ正しい順序", "平成31年4月〜令和元年5月", nil},
		{"全角数字でも比べる", "２０２１年４月～２０２０年３月", []string{"２０２１年４月～２０２０年３月"}},
		{"同じ年で月が片方に無ければ指摘しない", "2020年4月〜2020年", nil},
	})
}

func TestCountChars(t *testing.T) {
	for _, tc := range []struct {
		text string
		want int
	}{
		{"  あいう\nえお  ", 5},
		{"abc\r\n　de　", 6}, // 文中の全角空白は数え、末尾の全角空白は落とす
		{"", 0},
	} {
		if got := textcheck.CountChars(tc.text); got != tc.want {
			t.Errorf("CountChars(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

func TestCharLimit(t *testing.T) {
	for _, tc := range []struct {
		name         string
		limit        int
		mode         string
		chars        int
		wantSeverity string // "" は指摘なし
	}{
		{"within: 下限(8)未満は不足", 10, "within", 7, textcheck.SeverityWarning},
		{"within: 下限ちょうど", 10, "within", 8, ""},
		{"within: 上限ちょうど", 10, "within", 10, ""},
		{"within: 上限(10)超えは超過", 10, "within", 11, textcheck.SeverityCritical},
		{"around: 下限(9)未満は不足", 10, "around", 8, textcheck.SeverityWarning},
		{"around: 下限ちょうど", 10, "around", 9, ""},
		{"around: 上限ちょうど", 10, "around", 11, ""},
		{"around: 上限(11)超えは超過", 10, "around", 12, textcheck.SeverityCritical},
		{"未知のモードは within 扱い", 10, "unknown", 11, textcheck.SeverityCritical},
		{"字数指定なしは見ない", 0, "within", 100, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Repeat("あ", tc.chars)
			fs := textcheck.CharLimit{Limit: tc.limit, Mode: tc.mode}.Check(text)
			got := ""
			if len(fs) == 1 {
				got = fs[0].Severity
				if fs[0].Rule != "char_limit" || fs[0].Start != 0 || fs[0].End != tc.chars {
					t.Errorf("指摘の項目が違う: %+v", fs[0])
				}
			} else if len(fs) > 1 {
				t.Fatalf("指摘が %d 件ある: %+v", len(fs), fs)
			}
			if got != tc.wantSeverity {
				t.Errorf("severity = %q, want %q", got, tc.wantSeverity)
			}
		})
	}
}

func TestRun_SortsBySeverityThenPosition(t *testing.T) {
	text := strings.Repeat("あ", 20) + "５と3と4"
	fs := textcheck.Run(text, textcheck.MixedWidth{}, textcheck.CharLimit{Limit: 10, Mode: "within"})
	if len(fs) != 2 || fs[0].Rule != "char_limit" || fs[1].Rule != "mixed_width" {
		t.Fatalf("重要度順に並んでいない: %+v", fs)
	}
}
