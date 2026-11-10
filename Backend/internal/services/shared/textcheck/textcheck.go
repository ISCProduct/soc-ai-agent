// Package textcheck は ES・履歴書の形式を、LLM を使わずルールだけで検査する（#1530）。
//
// 検査は入力テキストだけで決まる純関数で、同じ入力には常に同じ指摘を返す。
// 誤字脱字の検出は辞書が要るため、初期範囲には含めない（DesignDoc）。
//
// 「混在」系のルール（mixed_width / mixed_numeral / mixed_era / mixed_style）は
// 除外リストを持たず、共起で判定する。同じ対象が両方の表記で現れたときだけ、
// 少ない側の表記を指摘する。除外リストはその場で辞書になり、際限が無いため。
//
// 位置（Finding.Start / End）は、Check に渡した文字列を UTF-16 のコード単位で数える。
// フロント（JavaScript）の文字列の添字と同じ単位なので、絵文字を含んでもずれない。
package textcheck

import "sort"

// 重要度は履歴書の LLM 指摘（resume.aiReviewItem.Severity）と同じ3値を使う。
// 表示は frontend/app/resume/utils.ts の getSeverityConfig が持っている。
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// Finding は1件の指摘。Start / End は UTF-16 単位の位置（End は含まない）。
type Finding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Message  string `json:"message"`
	Fix      string `json:"fix"`
	// PageHint / BlockIndex は履歴書のブロック位置で、resume.aiReviewItem と同じ意味・同じ JSON 名。
	// 検査は位置を知らないので、呼び出し側（履歴書レビュー）が埋める。
	PageHint   int `json:"page_hint,omitempty"`
	BlockIndex int `json:"block_index,omitempty"`
}

// Rule は1つの検査ルール。Name は Finding.Rule と同じ識別子を返す。
type Rule interface {
	Name() string
	Check(text string) []Finding
}

// DefaultRules は ES で使う全ルールを返す。charLimit が0以下なら char_limit は何も指摘しない。
// 呼び出し側でルールを並べると、1つ抜けてもテストで気付けないため、ここに集める。
func DefaultRules(charLimit int, mode string) []Rule {
	return []Rule{
		MixedWidth{},
		MixedNumeral{},
		MixedEra{},
		MixedStyle{},
		SpokenLanguage{},
		DateOrder{},
		CharLimit{Limit: charLimit, Mode: mode},
	}
}

var severityRank = map[string]int{SeverityCritical: 0, SeverityWarning: 1, SeverityInfo: 2}

// Run は rules をすべて適用し、重要度順（同じなら出現位置順）に並べて返す。
// 「重要度順で上位 N 件に絞る」のは表示側で行う。
func Run(text string, rules ...Rule) []Finding {
	var out []Finding
	for _, r := range rules {
		out = append(out, r.Check(text)...)
	}
	sortFindings(out)
	return out
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if a, b := severityRank[fs[i].Severity], severityRank[fs[j].Severity]; a != b {
			return a < b
		}
		return fs[i].Start < fs[j].Start
	})
}

// span は UTF-16 単位の [start, end)。
type span struct{ start, end int }

// positions はバイト位置・rune 位置を UTF-16 位置に引く表。
// 一致のたびに先頭から数え直すと入力長の2乗で遅くなるため、1回だけ作って引く。
type positions struct {
	byByte []int // バイト位置 → UTF-16 位置（rune の先頭と末尾だけ有効）
	byRune []int // rune 位置 → UTF-16 位置
}

func newPositions(s string) positions {
	p := positions{byByte: make([]int, len(s)+1), byRune: make([]int, 0, len(s)+1)}
	u := 0
	for i, r := range s {
		p.byByte[i] = u
		p.byRune = append(p.byRune, u)
		if r >= 0x10000 {
			u += 2 // サロゲートペア
		} else {
			u++
		}
	}
	p.byByte[len(s)] = u
	p.byRune = append(p.byRune, u)
	return p
}

func (p positions) bytes(b, e int) span { return span{p.byByte[b], p.byByte[e]} }
func (p positions) runes(i, j int) span { return span{p.byRune[i], p.byRune[j]} }
func (p positions) length() int         { return p.byRune[len(p.byRune)-1] }

// pickMinority は表記 a・b の出現から、指摘する側（少ない側）を返す。
// 同数なら a を指摘する。どちらかが0件なら混在していないので nil を返す。
func pickMinority(a, b []span) (flagged []span, flaggedA bool) {
	if len(a) == 0 || len(b) == 0 {
		return nil, false
	}
	if len(a) <= len(b) {
		return a, true
	}
	return b, false
}
