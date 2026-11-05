package es

import (
	"encoding/json"
	"net/http"
	"strings"
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

// 字数指定の有無と mode がルールまで渡ること（11字・10字指定は within なら超過、around なら範囲内）。
func TestFormatFindings_PassesCharLimitAndMode(t *testing.T) {
	text := strings.Repeat("あ", 11)
	limit := 10
	for _, tc := range []struct {
		name  string
		limit *int
		mode  string
		want  int
	}{
		{"字数指定なし", nil, "", 0},
		{"within は超過", &limit, "within", 1},
		{"around は範囲内", &limit, "around", 0},
		{"mode 未指定は within 扱い", &limit, "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := formatFindings(text, tc.limit, tc.mode)
			if fs == nil || len(fs) != tc.want {
				t.Fatalf("formatFindings = %+v, want %d 件（nil ではなく空配列）", fs, tc.want)
			}
		})
	}
}

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

// 位置は利用者が送った原文（trim 前）を基準にする。先頭の改行・空白（3単位）の分だけずれない。
func TestReview_PositionsAreBasedOnUntrimmedText(t *testing.T) {
	srv := ragStub(t, http.StatusOK, ragOKBody, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	ctx, rec := newCtx(http.MethodPost, "/api/es/review", `{"es_text": "\n  ITとＩＴ"}`)
	if err := (&ESReviewController{openaiClient: noopClient()}).Review(ctx); err != nil {
		t.Fatalf("Review: %v", err)
	}
	got := decodeBody(t, rec.Body.Bytes())
	arr, _ := got["format_findings"].([]any)
	if len(arr) != 1 {
		t.Fatalf("format_findings = %v, want 1件", got["format_findings"])
	}
	f := arr[0].(map[string]any)
	if f["start"] != float64(6) || f["end"] != float64(8) {
		t.Errorf("位置 = %v〜%v, want 6〜8（trim 前の原文で「ＩＴ」の位置）", f["start"], f["end"])
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

// RAG が 200 で JSON オブジェクト以外を返しても、500 にせず RAG の応答をそのまま返す。
func TestReview_MalformedRAGBodyIsPassedThrough(t *testing.T) {
	srv := ragStub(t, http.StatusOK, `[1, 2]`, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	ctx, rec := newCtx(http.MethodPost, "/api/es/review", `{"es_text": "`+mixedStyleText+`"}`)
	if err := (&ESReviewController{openaiClient: noopClient()}).Review(ctx); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200（素通し）", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "format_findings") || !strings.Contains(rec.Body.String(), "1") {
		t.Errorf("RAG の応答がそのまま返っていない: %s", rec.Body.String())
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

// リライトでも字数指定と mode が形式チェックまで渡ること。
func TestRewrite_PassesCharLimitModeToFormatCheck(t *testing.T) {
	text := strings.Repeat("あ", 11)
	for _, tc := range []struct {
		mode string
		want []string
	}{
		{"within", []string{"char_limit"}},
		{"around", []string{}},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			srv := ragStub(t, http.StatusOK, ragOKBody, nil)
			defer srv.Close()
			t.Setenv("RAG_REVIEW_URL", srv.URL)

			ctx, rec := newCtx(http.MethodPost, "/api/es/rewrite",
				`{"original_text": "`+text+`", "char_limit": 10, "char_limit_mode": "`+tc.mode+`"}`)
			if err := (&ESRewriteController{openaiClient: noopClient()}).Rewrite(ctx); err != nil {
				t.Fatalf("Rewrite: %v", err)
			}
			got := findingRules(t, decodeBody(t, rec.Body.Bytes())["format_findings"])
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("format_findings の rule = %v, want %v", got, tc.want)
			}
		})
	}
}

// リライトでも、位置は利用者が送った原文（trim 前）を基準にする。先頭の改行・空白（3単位）の分だけずれない。
func TestRewrite_PositionsAreBasedOnUntrimmedText(t *testing.T) {
	srv := ragStub(t, http.StatusOK, ragOKBody, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	ctx, rec := newCtx(http.MethodPost, "/api/es/rewrite", `{"original_text": "\n  ITとＩＴ"}`)
	if err := (&ESRewriteController{openaiClient: noopClient()}).Rewrite(ctx); err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	arr, _ := decodeBody(t, rec.Body.Bytes())["format_findings"].([]any)
	if len(arr) != 1 {
		t.Fatalf("format_findings = %v, want 1件", arr)
	}
	f := arr[0].(map[string]any)
	if f["start"] != float64(6) || f["end"] != float64(8) {
		t.Errorf("位置 = %v〜%v, want 6〜8（trim 前の原文で「ＩＴ」の位置）", f["start"], f["end"])
	}
}

// 添削でも、字数指定の mode がハンドラから形式チェックまで渡ること（11字・10字指定は within なら超過、around なら範囲内）。
func TestReview_PassesCharLimitModeToFormatCheck(t *testing.T) {
	text := strings.Repeat("あ", 11)
	for _, tc := range []struct {
		mode string
		want []string
	}{
		{"within", []string{"char_limit"}},
		{"around", []string{}},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			srv := ragStub(t, http.StatusOK, ragOKBody, nil)
			defer srv.Close()
			t.Setenv("RAG_REVIEW_URL", srv.URL)

			ctx, rec := newCtx(http.MethodPost, "/api/es/review",
				`{"es_text": "`+text+`", "char_limit": 10, "char_limit_mode": "`+tc.mode+`"}`)
			if err := (&ESReviewController{openaiClient: noopClient()}).Review(ctx); err != nil {
				t.Fatalf("Review: %v", err)
			}
			got := findingRules(t, decodeBody(t, rec.Body.Bytes())["format_findings"])
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("format_findings の rule = %v, want %v", got, tc.want)
			}
		})
	}
}
