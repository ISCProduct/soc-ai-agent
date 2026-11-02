package es

import (
	"encoding/json"
	"net/http"
	"testing"

	"Backend/internal/openai"
)

func noopClient() *openai.Client { return &openai.Client{OnUsage: func(openai.Usage) {}} }

func decodeBody(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("レスポンスの解析に失敗: %v（%s）", err, b)
	}
	return m
}

// findingRules は format_findings の各指摘の rule を順に返す。
func findingRules(t *testing.T, v any) []string {
	t.Helper()
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("format_findings が配列ではない: %#v", v)
	}
	out := []string{}
	for _, f := range arr {
		out = append(out, f.(map[string]any)["rule"].(string))
	}
	return out
}

// 敬体2文・常体1文で、常体の1文が mixed_style で指摘される。25字なので 10字指定なら超過。
const mixedStyleText = "私は部長です。毎日練習した。大会に出場しました。"

// #1530: 添削は RAG の応答を素通しする経路なので、型に無い項目を落とさずに format_findings だけを足す。
func TestReview_AddsFormatFindingsAndKeepsRAGFields(t *testing.T) {
	srv := ragStub(t, http.StatusOK, ragOKBody, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	ctx, rec := newCtx(http.MethodPost, "/api/es/review", `{"es_text": "`+mixedStyleText+`", "char_limit": 10}`)
	if err := (&ESReviewController{openaiClient: noopClient()}).Review(ctx); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := decodeBody(t, rec.Body.Bytes())
	if got["feedback"] != "フィードバック" || got["company_context_source"] != "none" {
		t.Errorf("RAG の項目が落ちた: %v", got)
	}
	if _, ok := got["usage"]; ok {
		t.Errorf("usage は取り除かれるはず: %v", got["usage"])
	}
	if rules := findingRules(t, got["format_findings"]); len(rules) != 2 || rules[0] != "char_limit" || rules[1] != "mixed_style" {
		t.Errorf("format_findings の rule = %v, want [char_limit mixed_style]（重要度順）", rules)
	}
}

// 指摘が無いときも null ではなく [] を返す（フロントで配列として扱えるように）。
func TestReview_NoFindingsIsEmptyArray(t *testing.T) {
	srv := ragStub(t, http.StatusOK, ragOKBody, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	ctx, rec := newCtx(http.MethodPost, "/api/es/review", `{"es_text": "私は部長です。"}`)
	if err := (&ESReviewController{openaiClient: noopClient()}).Review(ctx); err != nil {
		t.Fatalf("Review: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("レスポンスの解析に失敗: %v", err)
	}
	if string(raw["format_findings"]) != "[]" {
		t.Errorf("format_findings = %s, want []", raw["format_findings"])
	}
}

// RAG のエラー（422 の案内文など）は、形式チェックを足さずにそのまま返す。
func TestReview_RAGErrorIsForwardedWithoutFindings(t *testing.T) {
	srv := ragStub(t, http.StatusUnprocessableEntity, `{"detail": "文章が長すぎます"}`, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	ctx, rec := newCtx(http.MethodPost, "/api/es/review", `{"es_text": "`+mixedStyleText+`"}`)
	if err := (&ESReviewController{openaiClient: noopClient()}).Review(ctx); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d, want 422", rec.Code)
	}
	got := decodeBody(t, rec.Body.Bytes())
	if got["detail"] != "文章が長すぎます" {
		t.Errorf("RAG の案内文が変わった: %v", got)
	}
	if _, ok := got["format_findings"]; ok {
		t.Errorf("エラー時に format_findings を足してはいけない: %v", got)
	}
}

// リライトは改善文ではなく学生の原文を検査する（改善文「改善後の文章」には指摘が出ない）。
func TestRewrite_AddsFormatFindingsForOriginalText(t *testing.T) {
	srv := ragStub(t, http.StatusOK, ragOKBody, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	ctx, rec := newCtx(http.MethodPost, "/api/es/rewrite", `{"original_text": "`+mixedStyleText+`"}`)
	if err := (&ESRewriteController{openaiClient: noopClient()}).Rewrite(ctx); err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := decodeBody(t, rec.Body.Bytes())
	if got["rewritten_text"] != "改善後の文章" {
		t.Errorf("rewritten_text = %v", got["rewritten_text"])
	}
	if rules := findingRules(t, got["format_findings"]); len(rules) != 1 || rules[0] != "mixed_style" {
		t.Errorf("format_findings の rule = %v, want [mixed_style]", rules)
	}
}
