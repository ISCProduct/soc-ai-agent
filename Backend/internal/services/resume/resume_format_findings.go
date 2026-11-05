package resume

import (
	"Backend/internal/models"
	"Backend/internal/services/shared/textcheck"
)

// FormatFinding は履歴書の形式チェックの指摘（#1530）。
// 履歴書のブロック番号は 0 から始まるので、page_hint と block_index は 0 でも省略せずに出す
// （textcheck.Finding は ES 向けに omitempty。外側の同名フィールドが JSON では優先される）。
// Start/End は、そのブロックの Text の中の位置（UTF-16）。保存はしない（#1531）。
type FormatFinding struct {
	textcheck.Finding
	PageHint   int `json:"page_hint"`
	BlockIndex int `json:"block_index"`
}

// formatFindingsFor は、抽出したブロックに履歴書用の形式チェックをかける。指摘が無くても nil ではなく空を返す。
func formatFindingsFor(blocks []models.ResumeTextBlock) []FormatFinding {
	tb := make([]textcheck.Block, len(blocks))
	for i, b := range blocks {
		tb[i] = textcheck.Block{Text: b.Text, PageNumber: b.PageNumber, BlockIndex: b.BlockIndex}
	}
	fs := textcheck.CheckResume(tb)
	out := make([]FormatFinding, 0, len(fs))
	for _, f := range fs {
		out = append(out, FormatFinding{Finding: f, PageHint: f.PageHint, BlockIndex: f.BlockIndex})
	}
	return out
}
