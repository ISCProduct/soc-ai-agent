package resume

import (
	"strings"
	"unicode/utf16"

	"Backend/internal/models"
	"Backend/internal/services/shared/textcheck"
)

// FormatFinding は履歴書の形式チェックの指摘（#1530）。
// 履歴書のブロック番号は 0 から始まるので、page_hint と block_index は 0 でも省略せずに出す
// （textcheck.Finding は ES 向けに omitempty。外側の同名フィールドが JSON では優先される）。
// Start/End は、そのブロックの Text の中の位置（UTF-16）。保存はしない（#1531）。
//
// PageHint / BlockIndex は埋め込んだ Finding にもあり、JSON に出るのは外側だけ。
// 両方を同じ値にそろえるため、formatFindingsFor 以外では作らない。
type FormatFinding struct {
	textcheck.Finding
	PageHint   int `json:"page_hint"`
	BlockIndex int `json:"block_index"`
	// Excerpt は該当箇所の文字列。画面はブロックの本文を持たないので、ここで切り出して渡す（#1973 のレビュー）。
	Excerpt string `json:"excerpt"`
}

// formatExcerptMax は Excerpt の最大の文字数（ルーン）。超えたら切って「…」を付ける。
const formatExcerptMax = 30

type blockKey struct{ page, index int }

// formatFindingsFor は、抽出したブロックに履歴書用の形式チェックをかける。指摘が無くても nil ではなく空を返す。
func formatFindingsFor(blocks []models.ResumeTextBlock) []FormatFinding {
	tb := make([]textcheck.Block, len(blocks))
	texts := make(map[blockKey]string, len(blocks))
	for i, b := range blocks {
		tb[i] = textcheck.Block{Text: b.Text, PageNumber: b.PageNumber, BlockIndex: b.BlockIndex}
		texts[blockKey{b.PageNumber, b.BlockIndex}] = b.Text
	}
	fs := textcheck.CheckResume(tb)
	out := make([]FormatFinding, 0, len(fs))
	for _, f := range fs {
		out = append(out, FormatFinding{
			Finding:    f,
			PageHint:   f.PageHint,
			BlockIndex: f.BlockIndex,
			Excerpt:    formatExcerpt(texts[blockKey{f.PageHint, f.BlockIndex}], f.Start, f.End),
		})
	}
	return out
}

// formatExcerpt は text の UTF-16 の位置 [start, end) を切り出し、空白を1つにまとめる。
// 位置が範囲の外なら空文字を返す。
func formatExcerpt(text string, start, end int) string {
	u := utf16.Encode([]rune(text))
	if start < 0 || end > len(u) || start >= end {
		return ""
	}
	s := strings.Join(strings.Fields(string(utf16.Decode(u[start:end]))), " ")
	if rs := []rune(s); len(rs) > formatExcerptMax {
		s = string(rs[:formatExcerptMax]) + "…"
	}
	return s
}
