package es

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/openai"
	"Backend/internal/usagectx"

	"github.com/labstack/echo/v4"
)

// ragStub は RAG の /es/review を模す。受け取ったリクエスト本文を記録する。
func ragStub(t *testing.T, status int, body string, gotBody *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/es/review" {
			t.Errorf("RAG のパスが /es/review ではない: %s", r.URL.Path)
		}
		if gotBody != nil {
			if err := json.NewDecoder(r.Body).Decode(gotBody); err != nil {
				t.Errorf("リクエスト本文の解析に失敗: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func newCtx(method, path, body string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	return echo.New().NewContext(req, rec), rec
}

const ragOKBody = `{
  "specificity_score": 7,
  "star_score": 6,
  "company_fit_score": null,
  "length_balance_score": 5,
  "feedback": "フィードバック",
  "improved_text": "改善後の文章",
  "company_strategy": null,
  "company_context_source": "none",
  "star": {"situation": "状況", "task": "課題", "action": "施策", "result": "成果"},
  "improved_text_length": 6,
  "char_limit_satisfied": true,
  "usage": {"model": "gpt-4o", "prompt_tokens": 100, "completion_tokens": 200, "calls": 2}
}`

// #1533: リライトは RAG の /es/review の結果を rewritten_text / star に成形して返す。
func TestRewrite_UsesRAGReviewAndKeepsResponseShape(t *testing.T) {
	var ragReq map[string]any
	srv := ragStub(t, http.StatusOK, ragOKBody, &ragReq)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	var usages []openai.Usage
	cli := &openai.Client{OnUsage: func(u openai.Usage) { usages = append(usages, u) }}
	ctx, rec := newCtx(http.MethodPost, "/api/es/rewrite", `{
		"original_text": "元の文章", "question_type": "学チカ",
		"tech_stack": "Go, React", "company_name": "株式会社Example",
		"char_limit": 400, "char_limit_mode": "around"
	}`)

	if err := NewESRewriteController(cli).Rewrite(ctx); err != nil {
		t.Fatalf("Rewrite が失敗: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	// RAG へは es_text として渡し、リライト固有の入力も落とさない
	if ragReq["es_text"] != "元の文章" {
		t.Errorf("es_text = %v, want 元の文章", ragReq["es_text"])
	}
	if ragReq["tech_stack"] != "Go, React" {
		t.Errorf("tech_stack = %v", ragReq["tech_stack"])
	}
	if ragReq["char_limit"] != float64(400) || ragReq["char_limit_mode"] != "around" {
		t.Errorf("char_limit = %v / %v", ragReq["char_limit"], ragReq["char_limit_mode"])
	}

	var got esRewriteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("レスポンスの解析に失敗: %v", err)
	}
	if got.RewrittenText != "改善後の文章" {
		t.Errorf("rewritten_text = %q", got.RewrittenText)
	}
	if got.Star.Situation != "状況" || got.Star.Result != "成果" {
		t.Errorf("star = %+v", got.Star)
	}
	if got.ImprovedTextLength != 6 || got.CharLimitSatisfied == nil || !*got.CharLimitSatisfied {
		t.Errorf("字数の結果が落ちている: %+v", got)
	}
	// usage は内部情報。公開APIの本文には出さない
	if strings.Contains(rec.Body.String(), "usage") || strings.Contains(rec.Body.String(), "prompt_tokens") {
		t.Errorf("usage がレスポンスに漏れている: %s", rec.Body.String())
	}
	// コストは es_rewrite として記録される（#1294 の機能別内訳を維持）
	if len(usages) != 1 {
		t.Fatalf("usage 記録数 = %d, want 1", len(usages))
	}
	if usages[0].Feature != usagectx.FeatureESRewrite {
		t.Errorf("feature = %q, want %q", usages[0].Feature, usagectx.FeatureESRewrite)
	}
	if usages[0].PromptTokens != 100 || usages[0].CompletionTokens != 200 || usages[0].Model != "gpt-4o" {
		t.Errorf("記録したトークン量が違う: %+v", usages[0])
	}
}

// #1533: 添削は RAG のレスポンスをそのまま転送するが、usage だけは取り除く。
func TestReview_ForwardsBodyAndStripsUsage(t *testing.T) {
	srv := ragStub(t, http.StatusOK, ragOKBody, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	var usages []openai.Usage
	cli := &openai.Client{OnUsage: func(u openai.Usage) { usages = append(usages, u) }}
	ctx, rec := newCtx(http.MethodPost, "/api/es/review", `{"es_text": "元の文章", "char_limit": 400}`)

	if err := NewESReviewController(cli).Review(ctx); err != nil {
		t.Fatalf("Review が失敗: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("レスポンスの解析に失敗: %v", err)
	}
	if _, ok := got["usage"]; ok {
		t.Errorf("usage がレスポンスに漏れている: %s", rec.Body.String())
	}
	for _, key := range []string{"feedback", "improved_text", "star", "improved_text_length", "char_limit_satisfied", "company_context_source"} {
		if _, ok := got[key]; !ok {
			t.Errorf("%s が転送されていない: %s", key, rec.Body.String())
		}
	}
	if len(usages) != 1 || usages[0].Feature != usagectx.FeatureESReview {
		t.Errorf("es_review としてコストが記録されていない: %+v", usages)
	}
}

// #1521 の 422 案内文は、リライト経路でも利用者へ届く必要がある。
func TestRewrite_ForwardsRAGErrorStatusAndDetail(t *testing.T) {
	srv := ragStub(t, http.StatusUnprocessableEntity,
		`{"detail": "文章が長すぎて添削できませんでした。文字数を減らしてお試しください。"}`, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	ctx, rec := newCtx(http.MethodPost, "/api/es/rewrite", `{"original_text": "長い文章"}`)
	if err := NewESRewriteController(nil).Rewrite(ctx); err != nil {
		t.Fatalf("Rewrite が失敗: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "文字数を減らして") {
		t.Errorf("案内文が落ちている: %s", rec.Body.String())
	}
}

// RAG 未設定はリライトでも 503（プロンプトを持たないので Backend 単独では動けない）。
func TestRewrite_MissingRAGURL(t *testing.T) {
	t.Setenv("RAG_REVIEW_URL", "")
	ctx, _ := newCtx(http.MethodPost, "/api/es/rewrite", `{"original_text": "元の文章"}`)

	err := NewESRewriteController(nil).Rewrite(ctx)
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want 503 HTTPError", err)
	}
}

// usage を返さない RAG（旧バージョン）でも本文をそのまま返し、記録だけ諦める。
func TestReview_WithoutUsageKeepsBody(t *testing.T) {
	srv := ragStub(t, http.StatusOK, `{"feedback": "ok", "improved_text": "改善"}`, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	var usages []openai.Usage
	cli := &openai.Client{OnUsage: func(u openai.Usage) { usages = append(usages, u) }}
	ctx, rec := newCtx(http.MethodPost, "/api/es/review", `{"es_text": "元の文章"}`)

	if err := NewESReviewController(cli).Review(ctx); err != nil {
		t.Fatalf("Review が失敗: %v", err)
	}
	if !strings.Contains(rec.Body.String(), "改善") {
		t.Errorf("本文が落ちている: %s", rec.Body.String())
	}
	if len(usages) != 0 {
		t.Errorf("usage が無いのに記録された: %+v", usages)
	}
}
