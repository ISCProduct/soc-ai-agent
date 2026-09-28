package resume

// 引用(quote)と本文ブロックの照合テスト（Issue #1522）
// 実行: cd Backend && go test ./internal/services/resume/... -run TestMapReviewItems -v

import (
	"fmt"
	"testing"

	"Backend/internal/models"
)

// makeBlocks はテスト用の本文ブロックを組み立てる。
// BBox はどのブロックに紐づいたかを判定する目印として使う。
func makeBlocks(texts ...string) []models.ResumeTextBlock {
	blocks := make([]models.ResumeTextBlock, 0, len(texts))
	for i, text := range texts {
		blocks = append(blocks, models.ResumeTextBlock{
			PageNumber: 1,
			BlockIndex: i + 1,
			Text:       text,
			BBox:       fmt.Sprintf("bbox-%d", i+1),
		})
	}
	return blocks
}

const (
	blockPR      = "アルバイトでは接客を通じて売上向上に貢献しました。"
	blockWeb     = "Webサイトの制作を担当しました。"
	blockMotive  = "貴社の地域密着型のサービスに魅力を感じ志望しました。"
	blockLicense = "資格は日商簿記2級を取得しています、実務でも活用したいです。"
)

func TestMapReviewItemsQuoteMatching(t *testing.T) {
	standard := makeBlocks(blockPR, blockWeb, blockMotive, blockLicense)
	// 表記がほぼ同じで区別できないブロック
	ambiguous := makeBlocks(
		"Webサイトの制作を担当しアクセス数を伸ばしました。",
		"Webサイトの制作を担当しアクセス数を維持しました。",
	)

	tests := []struct {
		name     string
		blocks   []models.ResumeTextBlock
		item     aiReviewItem
		wantBBox string // 空文字なら紐づかないことを期待する
	}{
		{
			name:     "引用が本文と完全一致",
			blocks:   standard,
			item:     aiReviewItem{Quote: blockMotive, Message: "m"},
			wantBBox: "bbox-3",
		},
		{
			name:     "引用が要約・省略されている",
			blocks:   standard,
			item:     aiReviewItem{Quote: "接客を通じて売上向上に貢献した", Message: "m"},
			wantBBox: "bbox-1",
		},
		{
			name:     "全角半角の違い",
			blocks:   standard,
			item:     aiReviewItem{Quote: "Ｗｅｂサイトの制作を担当しました。", Message: "m"},
			wantBBox: "bbox-2",
		},
		{
			name:     "引用に改行が入っている",
			blocks:   standard,
			item:     aiReviewItem{Quote: "貴社の地域密着型のサービスに\n魅力を感じ志望しました。", Message: "m"},
			wantBBox: "bbox-3",
		},
		{
			name:     "約物(読点)の違い",
			blocks:   standard,
			item:     aiReviewItem{Quote: "資格は日商簿記2級を取得しています実務でも活用したいです", Message: "m"},
			wantBBox: "bbox-4",
		},
		{
			name:     "候補が2つ近接していて曖昧なら紐づけない",
			blocks:   ambiguous,
			item:     aiReviewItem{Quote: "Webサイトの制作を担当し成果を出した", Message: "m"},
			wantBBox: "",
		},
		{
			name:     "全く無関係な引用は紐づけない",
			blocks:   standard,
			item:     aiReviewItem{Quote: "宇宙開発の研究室で衛星の姿勢制御を研究しました", Message: "m"},
			wantBBox: "",
		},
		{
			name:     "page_hintとblock_indexが一致すれば引用が弱くても紐づく",
			blocks:   standard,
			item:     aiReviewItem{Quote: "宇宙開発の研究", Message: "m", PageHint: 1, BlockIndex: 2},
			wantBBox: "bbox-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapReviewItems(tt.blocks, []aiReviewItem{tt.item})
			if tt.wantBBox == "" {
				if len(got) != 0 {
					t.Fatalf("紐づかないことを期待したが %d 件紐づいた (bbox=%s)", len(got), got[0].BBox)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("1件紐づくことを期待したが %d 件だった", len(got))
			}
			if got[0].BBox != tt.wantBBox {
				t.Errorf("BBox = %s, want %s", got[0].BBox, tt.wantBBox)
			}
			if got[0].Severity != "info" {
				t.Errorf("Severity = %s, want info（未指定時の既定値）", got[0].Severity)
			}
		})
	}
}

// 一文が複数行のブロックに分かれている場合は、引用と最も長く重なるブロックに紐づける。
func TestMapReviewItemsQuoteSpansBlocks(t *testing.T) {
	blocks := makeBlocks("学生時代はサークルの新人勧誘を担当し、", "50名の入会につなげました。")
	got := mapReviewItems(blocks, []aiReviewItem{{
		Quote:   "学生時代はサークルの新人勧誘を担当し、50名の入会につなげました。",
		Message: "m",
	}})
	if len(got) != 1 {
		t.Fatalf("1件紐づくことを期待したが %d 件だった", len(got))
	}
	if got[0].BBox != "bbox-1" {
		t.Errorf("BBox = %s, want bbox-1", got[0].BBox)
	}
}
