package textcheck

import (
	"sort"
	"strings"
	"unicode/utf16"
)

// ResumeRules は履歴書で使うルール（#1530）。履歴書は欄ごとに書き方が決まっていて、
// 全角半角・漢数字・和暦が欄によって混ざるのは自然なので、その3つの混在は見ない。字数制限は ES だけ。
func ResumeRules() []Rule {
	return []Rule{MixedStyle{}, DateOrder{}, SpokenLanguage{}}
}

// CheckResume は、ブロックを改行でつないだ全文に ResumeRules をかけ（文体の混在は履歴書全体で見る）、
// 指摘をブロックに戻す。Start/End はそのブロックの Text の中の位置（UTF-16）、
// PageHint/BlockIndex はそのブロックのもの。最後に空欄の指摘を加える。
func CheckResume(blocks []Block) []Finding {
	var b strings.Builder
	starts := make([]int, len(blocks))
	lens := make([]int, len(blocks))
	pos := 0
	for i, blk := range blocks {
		if i > 0 {
			b.WriteByte('\n')
			pos++
		}
		starts[i] = pos
		lens[i] = len(utf16.Encode([]rune(blk.Text)))
		pos += lens[i]
		b.WriteString(blk.Text)
	}

	var out []Finding
	for _, f := range Run(b.String(), ResumeRules()...) {
		k := sort.Search(len(starts), func(j int) bool { return starts[j] > f.Start }) - 1
		if k < 0 {
			continue
		}
		// つなぎの改行から始まる指摘は、次のブロックの先頭からにする。
		if f.Start >= starts[k]+lens[k] && k+1 < len(starts) {
			k++
			f.Start = starts[k]
		}
		f.Start -= starts[k]
		f.End -= starts[k]
		if f.End > lens[k] {
			f.End = lens[k] // ブロックをまたぐ指摘は、始まったブロックの中で切る
		}
		if f.Start >= f.End {
			continue
		}
		f.PageHint = blocks[k].PageNumber
		f.BlockIndex = blocks[k].BlockIndex
		out = append(out, f)
	}
	out = append(out, BlankFields(blocks)...)

	sort.SliceStable(out, func(i, j int) bool {
		a, c := out[i], out[j]
		if ra, rc := resumeSeverityRank(a.Severity), resumeSeverityRank(c.Severity); ra != rc {
			return ra < rc
		}
		if a.PageHint != c.PageHint {
			return a.PageHint < c.PageHint
		}
		if a.BlockIndex != c.BlockIndex {
			return a.BlockIndex < c.BlockIndex
		}
		return a.Start < c.Start
	})
	return out
}

func resumeSeverityRank(s string) int {
	switch s {
	case SeverityCritical:
		return 0
	case SeverityWarning:
		return 1
	}
	return 2
}
