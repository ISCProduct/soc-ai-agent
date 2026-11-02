// Package textcheck は ES・履歴書の形式を、LLM を使わずルールだけで検査する（#1530）。
//
// 検査は入力テキストだけで決まる純関数で、同じ入力には常に同じ指摘を返す。
// 誤字脱字の検出は辞書が要るため、初期範囲には含めない（DesignDoc）。
//
// 「混在」系のルール（mixed_width / mixed_numeral / mixed_era / mixed_style）は
// 除外リストを持たず、共起で判定する。同じ対象が両方の表記で現れたときだけ、
// 少ない側の表記を指摘する。除外リストはその場で辞書になり、際限が無いため。
// 「一つ」だけの文章は「1つ」が無いので指摘しない。
package textcheck

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// 重要度は履歴書の LLM 指摘（resume.aiReviewItem.Severity）と同じ3値を使う。
// 表示は frontend/app/resume/utils.ts の getSeverityConfig が持っている。
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// Finding は1件の指摘。Start / End は対象テキスト内の位置（rune 単位。End は含まない）。
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

// Rule は1つの検査ルール。
type Rule interface {
	Check(text string) []Finding
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

// span は rune 単位の [start, end)。
type span struct{ start, end int }

// runeSpan はバイト位置の範囲 [b, e) を rune 位置に直す。
func runeSpan(s string, b, e int) span {
	start := utf8.RuneCountInString(s[:b])
	return span{start, start + utf8.RuneCountInString(s[b:e])}
}

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

// normalizeDigits は全角数字を半角に置き換える。rune 数は変わらないので、
// 置き換え後の文字列で求めた rune 位置は、元の文字列でもそのまま使える。
func normalizeDigits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return '0' + (r - '０')
		}
		return r
	}, s)
}
