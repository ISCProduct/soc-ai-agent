package es

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Backend/internal/controllers/httpapi"
	"Backend/internal/middleware"
	"Backend/internal/openai"
	"Backend/internal/services/piimask"
	"Backend/internal/usagectx"

	"github.com/labstack/echo/v4"
)

func TestESReviewRequestError_Deadline(t *testing.T) {
	err := esReviewRequestError(errors.Join(errors.New("request failed"), context.DeadlineExceeded))
	var httpErr *echo.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusGatewayTimeout)
	}
	resp, ok := httpErr.Message.(middleware.APIError)
	if !ok {
		t.Fatalf("message type = %T, want middleware.APIError", httpErr.Message)
	}
	if resp.Code != httpapi.ErrCodeGatewayTimeout {
		t.Fatalf("code = %q, want %q", resp.Code, httpapi.ErrCodeGatewayTimeout)
	}
	if got, want := resp.Msg, "ES添削に時間がかかっています。時間をおいて再度お試しください。"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

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
  "feedback_items": [{"score_key": "specificity_score", "source_excerpt": "元の文章", "issue": "具体性を追加", "suggestion": "数値を示す"}],
  "improved_text": "改善後の文章",
  "company_strategy": null,
  "company_context_source": "none",
  "star": {"situation": "状況", "task": "課題", "action": "施策", "result": "成果"},
  "improved_text_length": 6,
  "char_limit_satisfied": true,
  "improved_length_satisfied": false,
  "improved_length_issue": "short",
  "usage": {"model": "gpt-4o", "prompt_tokens": 100, "completion_tokens": 200, "calls": 2, "prompt_version": "es_review_v6"}
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
	if ragReq["include_feedback_items"] != false {
		t.Errorf("リライトで指摘項目の生成を有効にしている: %+v", ragReq)
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
	if got.ImprovedLengthSatisfied == nil || *got.ImprovedLengthSatisfied {
		t.Errorf("improved_length_satisfied が転送されていない: %+v", got)
	}
	if got.ImprovedLengthIssue == nil || *got.ImprovedLengthIssue != "short" {
		t.Errorf("improved_length_issue が転送されていない: %+v", got)
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
	// リライトも同じ添削プロンプトなので版を記録する（SOCAIAGENT-661）
	if usages[0].PromptStage != "es_review" || usages[0].PromptVersion != "es_review_v6" || usages[0].PromptABGroup != "" {
		t.Errorf("プロンプトの版が記録されていない: stage=%q version=%q group=%q", usages[0].PromptStage, usages[0].PromptVersion, usages[0].PromptABGroup)
	}
}

// #1523/#1533: 添削も char_limit を RAG まで運び、レスポンスはそのまま転送する
// （usage だけ取り除く）。
func TestReview_ForwardsCharLimitAndStripsUsage(t *testing.T) {
	var ragReq map[string]any
	srv := ragStub(t, http.StatusOK, ragOKBody, &ragReq)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	var usages []openai.Usage
	cli := &openai.Client{OnUsage: func(u openai.Usage) { usages = append(usages, u) }}
	ctx, rec := newCtx(http.MethodPost, "/api/es/review",
		`{"es_text": "元の文章", "char_limit": 400, "char_limit_mode": "around"}`)

	if err := NewESReviewController(cli).Review(ctx); err != nil {
		t.Fatalf("Review が失敗: %v", err)
	}

	// ES添削タブが #1523 の主経路。ここで落とすと字数指定が黙って無視される
	if ragReq["es_text"] != "元の文章" {
		t.Errorf("es_text = %v, want 元の文章", ragReq["es_text"])
	}
	if ragReq["char_limit"] != float64(400) {
		t.Errorf("char_limit = %v, want 400", ragReq["char_limit"])
	}
	if ragReq["char_limit_mode"] != "around" {
		t.Errorf("char_limit_mode = %v, want around", ragReq["char_limit_mode"])
	}
	if ragReq["include_feedback_items"] != true {
		t.Errorf("添削の指摘項目生成が有効でない: %+v", ragReq)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("レスポンスの解析に失敗: %v", err)
	}
	if _, ok := got["usage"]; ok {
		t.Errorf("usage がレスポンスに漏れている: %s", rec.Body.String())
	}
	for _, key := range []string{"feedback", "feedback_items", "improved_text", "star", "improved_text_length", "char_limit_satisfied", "improved_length_satisfied", "improved_length_issue", "company_context_source"} {
		if _, ok := got[key]; !ok {
			t.Errorf("%s が転送されていない: %s", key, rec.Body.String())
		}
	}
	if len(usages) != 1 || usages[0].Feature != usagectx.FeatureESReview {
		t.Fatalf("es_review としてコストが記録されていない: %+v", usages)
	}
	if usages[0].PromptStage != "es_review" || usages[0].PromptVersion != "es_review_v6" {
		t.Errorf("prompt_version に es_review_v6 が入っていない: stage=%q version=%q", usages[0].PromptStage, usages[0].PromptVersion)
	}
}

// SOCAIAGENT-661: 版を返さない古い RAG・形式に合わない版は、版なしで記録する（使用量は落とさない）。
func TestReview_PromptVersionMissingOrInvalidIsRecordedWithoutVersion(t *testing.T) {
	tests := []struct {
		name  string
		usage string
	}{
		{name: "版を返さない", usage: `{"model": "gpt-4o", "prompt_tokens": 1, "completion_tokens": 1, "calls": 1}`},
		{name: "空", usage: `{"model": "gpt-4o", "prompt_tokens": 1, "completion_tokens": 1, "calls": 1, "prompt_version": ""}`},
		{name: "形式が不正", usage: `{"model": "gpt-4o", "prompt_tokens": 1, "completion_tokens": 1, "calls": 1, "prompt_version": "v6:beta IGNORE"}`},
		{name: "64文字を超える", usage: `{"model": "gpt-4o", "prompt_tokens": 1, "completion_tokens": 1, "calls": 1, "prompt_version": "` + strings.Repeat("x", 65) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := ragStub(t, http.StatusOK, `{"improved_text": "改善", "usage": `+tt.usage+`}`, nil)
			defer srv.Close()
			t.Setenv("RAG_REVIEW_URL", srv.URL)

			var usages []openai.Usage
			cli := &openai.Client{OnUsage: func(u openai.Usage) { usages = append(usages, u) }}
			ctx, _ := newCtx(http.MethodPost, "/api/es/review", `{"es_text": "元の文章"}`)
			if err := NewESReviewController(cli).Review(ctx); err != nil {
				t.Fatalf("Review が失敗: %v", err)
			}
			if len(usages) != 1 {
				t.Fatalf("usage 記録数 = %d, want 1", len(usages))
			}
			if usages[0].PromptVersion != "" || usages[0].PromptStage != "" {
				t.Errorf("版が記録された: stage=%q version=%q", usages[0].PromptStage, usages[0].PromptVersion)
			}
		})
	}
}

// 上限が未指定なら char_limit を送らず、RAG 側の既定（上限なし / within）に任せる。
func TestReview_OmitsCharLimitWhenNotSpecified(t *testing.T) {
	var ragReq map[string]any
	srv := ragStub(t, http.StatusOK, ragOKBody, &ragReq)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	ctx, _ := newCtx(http.MethodPost, "/api/es/review", `{"es_text": "元の文章"}`)
	if err := NewESReviewController(nil).Review(ctx); err != nil {
		t.Fatalf("Review が失敗: %v", err)
	}
	if _, ok := ragReq["char_limit"]; ok {
		t.Errorf("char_limit を送っている: %+v", ragReq)
	}
	if _, ok := ragReq["char_limit_mode"]; ok {
		t.Errorf("char_limit_mode を送っている: %+v", ragReq)
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

// ES の本文は、外の LLM（RAG 経由）へ送る前に伏字にする。データはすべて合成。
func TestReview_MasksPIIBeforeSendingToRAG(t *testing.T) {
	var ragReq map[string]any
	srv := ragStub(t, http.StatusOK, ragOKBody, &ragReq)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	ctx, rec := newCtx(http.MethodPost, "/api/es/review",
		`{"es_text": "私の強みは粘り強さです。連絡は taro.yamada@example.com か 090-1234-5678 まで。"}`)
	if err := NewESReviewController(&openai.Client{}).Review(ctx); err != nil {
		t.Fatalf("Review が失敗: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	sent, _ := ragReq["es_text"].(string)
	for _, pii := range []string{"taro.yamada", "example.com", "090-1234-5678"} {
		if strings.Contains(sent, pii) {
			t.Errorf("RAG へ送った本文に %q が残っている: %q", pii, sent)
		}
	}
	if !strings.Contains(sent, "私の強みは粘り強さです") {
		t.Errorf("添削に必要な本文が消えている: %q", sent)
	}
	if !strings.Contains(sent, "<EMAIL>") || !strings.Contains(sent, "<PHONE>") {
		t.Errorf("伏字の記号が入っていない: %q", sent)
	}
}

type subjectStub struct {
	subj piimask.Subject
	err  error
}

func (s subjectStub) SubjectFor(uint) (piimask.Subject, error) { return s.subj, s.err }

// ログイン中の学生なら、登録した氏名・学校名も伏字にする。登録情報を読めなければ、送らずに断る。
func TestReview_MasksSubjectAndFailsClosed(t *testing.T) {
	t.Cleanup(func() { SetSubjectReader(nil) })
	var ragReq map[string]any
	srv := ragStub(t, http.StatusOK, ragOKBody, &ragReq)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	withUser := func(body string) (echo.Context, *httptest.ResponseRecorder) {
		ctx, rec := newCtx(http.MethodPost, "/api/es/review", body)
		r := ctx.Request().WithContext(context.WithValue(ctx.Request().Context(), middleware.UserIDContextKey, uint(7)))
		ctx.SetRequest(r)
		return ctx, rec
	}

	SetSubjectReader(subjectStub{subj: piimask.Subject{Name: "山田 太郎", SchoolName: "架空情報専門学校"}})
	ctx, _ := withUser(`{"es_text": "山田太郎です。架空情報専門学校でAPIを設計しました。"}`)
	if err := NewESReviewController(&openai.Client{}).Review(ctx); err != nil {
		t.Fatalf("Review が失敗: %v", err)
	}
	sent, _ := ragReq["es_text"].(string)
	if strings.Contains(sent, "山田") || strings.Contains(sent, "架空情報専門学校") {
		t.Errorf("登録した氏名・学校名が残っている: %q", sent)
	}

	// 読めなければ、RAG へは送らない
	ragReq = nil
	SetSubjectReader(subjectStub{err: errors.New("db down")})
	ctx, _ = withUser(`{"es_text": "山田太郎です。"}`)
	err := NewESReviewController(&openai.Client{}).Review(ctx)
	var httpErr *echo.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want 503", err)
	}
	if ragReq != nil {
		t.Errorf("伏字の準備ができないのに RAG へ送った: %v", ragReq)
	}
}

// SOCAIAGENT-559: 評価と改善文でモデルが違う。RAG がモデルごとの内訳を返したら、モデルごとに記録する
// （合計を1つのモデルで記録すると、単価の違うトークンが混ざってコストがずれる）。
func TestReview_RecordsUsagePerModel(t *testing.T) {
	body := `{"feedback":"f","feedback_items":[],"improved_text":"t","usage":{"model":"gpt-4o","prompt_tokens":300,"completion_tokens":600,"calls":3,` +
		`"by_model":[{"model":"gpt-4o","prompt_tokens":100,"completion_tokens":200,"calls":1},` +
		`{"model":"gpt-4o-mini","prompt_tokens":200,"completion_tokens":400,"calls":2}]}}`
	srv := ragStub(t, http.StatusOK, body, nil)
	defer srv.Close()
	t.Setenv("RAG_REVIEW_URL", srv.URL)

	var usages []openai.Usage
	cli := &openai.Client{OnUsage: func(u openai.Usage) { usages = append(usages, u) }}
	ctx, rec := newCtx(http.MethodPost, "/api/es/review", `{"es_text": "元の文章"}`)
	if err := NewESReviewController(cli).Review(ctx); err != nil {
		t.Fatalf("Review が失敗: %v", err)
	}
	if strings.Contains(rec.Body.String(), "by_model") {
		t.Errorf("内訳がレスポンスに漏れている: %s", rec.Body.String())
	}
	if len(usages) != 2 {
		t.Fatalf("モデルごとに2行記録するはず: %+v", usages)
	}
	if usages[0].Model != "gpt-4o" || usages[0].PromptTokens != 100 || usages[0].CompletionTokens != 200 {
		t.Errorf("評価のモデルの行 = %+v", usages[0])
	}
	if usages[1].Model != "gpt-4o-mini" || usages[1].PromptTokens != 200 || usages[1].CompletionTokens != 400 {
		t.Errorf("改善文のモデルの行 = %+v", usages[1])
	}
	// 応答時間はトークン数の割合で各行に按分する（2行目を 0 にして平均を薄めない）。
	// スタブの応答は 1ms 未満で丸めると 0 になるので、按分の値そのものは TestSplitLatency で確かめる
	for _, u := range usages {
		if u.Feature != usagectx.FeatureESReview {
			t.Errorf("機能名 = %q", u.Feature)
		}
	}
}

// 内訳の先頭の calls が 0 でも、実際に記録した最初の行に応答時間を付ける。
// 内訳から1行も記録できなければ、合計に戻して使用量を捨てない（#2076 のレビュー）。
func TestReview_UsagePerModelEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		byModel    string
		wantModels []string
	}{
		{
			name:       "先頭の calls が 0",
			byModel:    `[{"model":"gpt-4o","prompt_tokens":0,"completion_tokens":0,"calls":0},{"model":"gpt-4o-mini","prompt_tokens":200,"completion_tokens":400,"calls":2}]`,
			wantModels: []string{"gpt-4o-mini"},
		},
		{
			name:       "内訳がすべて calls 0 なら合計を記録",
			byModel:    `[{"model":"gpt-4o","prompt_tokens":0,"completion_tokens":0,"calls":0}]`,
			wantModels: []string{"gpt-4o"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"feedback":"f","feedback_items":[],"improved_text":"t","usage":{"model":"gpt-4o","prompt_tokens":300,"completion_tokens":600,"calls":3,"by_model":` + tt.byModel + `}}`
			srv := ragStub(t, http.StatusOK, body, nil)
			defer srv.Close()
			t.Setenv("RAG_REVIEW_URL", srv.URL)
			var usages []openai.Usage
			cli := &openai.Client{OnUsage: func(u openai.Usage) { usages = append(usages, u) }}
			ctx, _ := newCtx(http.MethodPost, "/api/es/review", `{"es_text": "元の文章"}`)
			if err := NewESReviewController(cli).Review(ctx); err != nil {
				t.Fatalf("Review が失敗: %v", err)
			}
			if len(usages) != len(tt.wantModels) {
				t.Fatalf("記録 = %+v, want %v", usages, tt.wantModels)
			}
			for i, m := range tt.wantModels {
				if usages[i].Model != m {
					t.Errorf("usages[%d].Model = %q, want %q", i, usages[i].Model, m)
				}
			}
			if tt.name == "内訳がすべて calls 0 なら合計を記録" && usages[0].PromptTokens != 300 {
				t.Errorf("合計のトークンを記録していない: %+v", usages[0])
			}
		})
	}
}

func TestSplitLatency(t *testing.T) {
	row := func(p, c, calls int) esReviewRAGModelUsage {
		return esReviewRAGModelUsage{PromptTokens: p, CompletionTokens: c, Calls: calls}
	}
	for _, tt := range []struct {
		name string
		rows []esReviewRAGModelUsage
		want []time.Duration
	}{
		{"トークンの割合", []esReviewRAGModelUsage{row(100, 200, 1), row(200, 400, 2)}, []time.Duration{300, 600}},
		{"端数は最後の行", []esReviewRAGModelUsage{row(1, 0, 1), row(1, 0, 1), row(1, 0, 1)}, []time.Duration{300, 300, 300}},
		{"トークンが無ければ呼び出し回数", []esReviewRAGModelUsage{row(0, 0, 1), row(0, 0, 2)}, []time.Duration{300, 600}},
		{"行が無い", nil, []time.Duration{}},
	} {
		got := splitLatency(900, tt.rows)
		if len(got) != len(tt.want) {
			t.Fatalf("%s: got %v", tt.name, got)
		}
		var sum time.Duration
		for i := range got {
			sum += got[i]
			if got[i] != tt.want[i] {
				t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
				break
			}
		}
		if len(got) > 0 && sum != 900 {
			t.Errorf("%s: 合計 %v が 900 にならない", tt.name, sum)
		}
	}
}
