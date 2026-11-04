package resume

import (
	"strings"
	"testing"

	"Backend/internal/models"
)

// TestOCRRewriteMinConfidenceBoundary は書き換えを捨てる境目を上下から固定する。
// 0.599 は捨て、0.6 は残す。定数をこの外へ動かすと落ちる。
func TestOCRRewriteMinConfidenceBoundary(t *testing.T) {
	blocks := makeBlocks("アルバイトでは接客を通じて売上向上に貢献しました。")
	item := aiReviewItem{
		Quote:      blocks[0].Text,
		Message:    "具体性を足してください",
		Suggestion: "人数と期間を書く",
		Rewrite:    "3名の店舗で1年間、接客を通じて売上向上に貢献しました。",
		Severity:   "info",
	}

	low := 0.599
	blocks[0].Confidence = &low
	dropped := mapReviewItems(blocks, []aiReviewItem{item})
	if len(dropped) != 1 {
		t.Fatalf("紐づけ件数 = %d, want 1", len(dropped))
	}
	if dropped[0].Rewrite != "" || dropped[0].RewriteWithheld != rewriteWithheldLowOCR {
		t.Fatalf("下限未満なのに書き換えを残した: rewrite=%q withheld=%q", dropped[0].Rewrite, dropped[0].RewriteWithheld)
	}
	if dropped[0].Quote != item.Quote {
		t.Fatalf("原文 = %q, want %q", dropped[0].Quote, item.Quote)
	}

	high := OCRRewriteMinConfidence
	blocks[0].Confidence = &high
	kept := mapReviewItems(blocks, []aiReviewItem{item})
	if len(kept) != 1 || kept[0].Rewrite != item.Rewrite || kept[0].RewriteWithheld != "" {
		t.Fatalf("下限ちょうどで書き換えを捨てた: %+v", kept)
	}

	blocks[0].Confidence = nil
	unknown := mapReviewItems(blocks, []aiReviewItem{item})
	if len(unknown) != 1 || unknown[0].Rewrite != item.Rewrite {
		t.Fatalf("信頼度不明の行まで書き換えを捨てた: %+v", unknown)
	}
}

func TestNormalizeOCRScore_範囲外は不明(t *testing.T) {
	zero := normalizeOCRScore(0)
	if zero == nil || *zero != 0 {
		t.Fatal("0 は読めなかったスコアとして残す")
	}
	if normalizeOCRScore(1.5) != nil || normalizeOCRScore(-0.01) != nil {
		t.Fatal("0〜1 の外は不明にする")
	}
	if got := normalizeOCRScorePtr(nil); got != nil {
		t.Fatal("スコア無しを値にした")
	}
}

func TestBuildResumeText_低信頼行に印を付ける(t *testing.T) {
	low := 0.1
	blocks := []models.ResumeTextBlock{
		{PageNumber: 1, BlockIndex: 0, Text: "売上を伸ばしました", Confidence: &low},
		{PageNumber: 1, BlockIndex: 1, Text: "チームをまとめました"},
	}
	text := buildResumeText(blocks, 1000)
	if !strings.Contains(text, "[P1B0] "+lowConfidenceMark+" 売上を伸ばしました") {
		t.Fatalf("低信頼の印が無い: %q", text)
	}
	if strings.Contains(text, lowConfidenceMark+" チームをまとめました") {
		t.Fatalf("信頼度不明の行に印を付けた: %q", text)
	}

	prompt := buildReviewPrompt(text, "", "エンジニア", "", "new_grad")
	retry := buildReviewRetryPrompt(buildBlockList(blocks), "", "エンジニア", "", "new_grad")
	for name, body := range map[string]string{"初回": prompt, "やり直し": retry} {
		if !strings.Contains(body, lowConfidenceMark) || !strings.Contains(body, `"rewrite"`) {
			t.Fatalf("%sプロンプトに書き換えの契約が無い", name)
		}
	}
}

func TestBuildReviewPromptFromTextWithConfidence_印は低信頼行だけ(t *testing.T) {
	prompt := BuildReviewPromptFromTextWithConfidence(
		"読めない行です\n売上が伸びました",
		[]float64{0.1, 0.9},
		"", "職種", "", "new_grad",
	)
	if !strings.Contains(prompt, lowConfidenceMark+" 読めない行です") {
		t.Fatal("低信頼行に印が無い")
	}
	if strings.Contains(prompt, lowConfidenceMark+" 売上が伸びました") {
		t.Fatal("信頼できる行に印を付けた")
	}
	// 範囲外は不明。印を付けて書き換えを禁じない。
	// 指示文そのものには印の説明があるので、行の方だけ見る。
	outOfRange := BuildReviewPromptFromTextWithConfidence("売上が伸びました", []float64{1.5}, "", "職種", "", "new_grad")
	if strings.Contains(outOfRange, lowConfidenceMark+" 売上が伸びました") {
		t.Fatal("範囲外のスコアを低信頼として印を付けた")
	}
}

func TestAnnotationPayload_原文と書き換えを渡す(t *testing.T) {
	payload := annotationPayload([]models.ResumeReviewItem{{
		PageNumber:      1,
		BBox:            `{"bbox":[1,2,3,4],"page_width":100,"page_height":200}`,
		Severity:        "warning",
		Message:         "具体性",
		Suggestion:      "数値を入れる",
		Quote:           "売上を伸ばした",
		Rewrite:         "売上を前年比120%に伸ばした",
		RewriteWithheld: "",
	}})
	if len(payload) != 1 {
		t.Fatalf("件数 = %d", len(payload))
	}
	got := payload[0]
	if got["quote"] != "売上を伸ばした" || got["rewrite"] != "売上を前年比120%に伸ばした" {
		t.Fatalf("注釈へ原文か書き換えが渡っていない: %v", got)
	}
	bbox, _ := got["bbox"].([]float64)
	if len(bbox) != 4 || bbox[0] != 1 || got["page_width"] != 100.0 {
		t.Fatalf("bbox の分解が壊れた: %v", got)
	}
}
