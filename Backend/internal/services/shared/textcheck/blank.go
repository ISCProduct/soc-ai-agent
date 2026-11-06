package textcheck

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"
)

// Block は履歴書の1ブロック（PDF から取り出した文字のかたまり）。
// textcheck が models に依存しないよう、必要なものだけ持つ。
type Block struct {
	Text       string
	PageNumber int
	BlockIndex int
}

// requiredHeadings は、空欄なら指摘する見出し（#1530 で 氏名・生年月日・住所・志望動機・自己PR に決めた）。
var requiredHeadings = map[string]bool{"氏名": true, "生年月日": true, "住所": true, "志望動機": true, "自己PR": true}

// otherHeadings は空欄の指摘はしないが、見出しとして扱うもの。
// 必須の見出しの次がこれなら、必須の見出しの欄は空欄とみなす。
var otherHeadings = map[string]bool{
	"ふりがな": true, "フリガナ": true, "性別": true, "現住所": true, "連絡先": true,
	"電話": true, "電話番号": true, "携帯電話": true, "メール": true, "メールアドレス": true, "E-mail": true,
	"学歴": true, "職歴": true, "学歴・職歴": true, "資格": true, "免許・資格": true,
	"趣味": true, "特技": true, "趣味・特技": true, "本人希望記入欄": true,
}

// BlankFields は、必須の見出しと完全に一致するブロックの次（空のブロックは飛ばす）が、
// 無いか見出しなら、その欄を空欄として info で指摘する。見出しが見つからなければ何も出さない。
// Start/End は見出しのブロックの Text 全体（UTF-16）。
func BlankFields(blocks []Block) []Finding {
	var out []Finding
	for i, b := range blocks {
		h := normalizeHeading(b.Text)
		if !requiredHeadings[h] {
			continue
		}
		if next := nextNonEmptyBlock(blocks, i); next >= 0 && !isResumeHeading(blocks[next].Text) {
			continue
		}
		out = append(out, Finding{
			Rule:       "blank_field",
			Severity:   SeverityInfo,
			Start:      0,
			End:        len(utf16.Encode([]rune(b.Text))),
			Message:    fmt.Sprintf("「%s」の欄が空欄のようです。記入漏れがないか確認してください。", h),
			PageHint:   b.PageNumber,
			BlockIndex: b.BlockIndex,
		})
	}
	return out
}

// normalizeHeading は、見出しの比較の前に空白を除き、全角英数記号を半角にし、末尾の「:」を除く。
func normalizeHeading(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch {
		case unicode.IsSpace(c):
			continue
		case c >= '！' && c <= '～':
			c -= 0xFEE0
		}
		b.WriteRune(c)
	}
	return strings.TrimRight(b.String(), ":")
}

func isResumeHeading(text string) bool {
	h := normalizeHeading(text)
	return requiredHeadings[h] || otherHeadings[h]
}

func nextNonEmptyBlock(blocks []Block, i int) int {
	for j := i + 1; j < len(blocks); j++ {
		if strings.TrimSpace(blocks[j].Text) != "" {
			return j
		}
	}
	return -1
}
