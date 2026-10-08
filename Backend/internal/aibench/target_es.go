package aibench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"Backend/internal/ragclient"
)

// esTarget は ES添削（RAG の POST /es/review）を評価する。
//
// Go からプロンプトを再現せず HTTP で叩くのは、プロンプトが Python 側
// (rag/services/es_review.py) にあるため。Go に写すと本番と別物を測る。
// そのため RAG サービスの起動が必須で、起動していないと評価できない。
type esTarget struct {
	baseURL string
	model   string
}

// esReviewResponse は /es/review のレスポンス（rag/models.py の ESReviewResponse）。
//
// ポインタ型にしているキーは「モデルが返さなかった」と「null を返した」を
// 区別するため。company_fit_score は企業情報が無いとき null が正しい（#1524）。
type esReviewResponse struct {
	SpecificityScore   *int    `json:"specificity_score"`
	StarScore          *int    `json:"star_score"`
	CompanyFitScore    *int    `json:"company_fit_score"`
	LengthBalanceScore *int    `json:"length_balance_score"`
	Feedback           string  `json:"feedback"`
	ImprovedText       string  `json:"improved_text"`
	CompanyStrategy    *string `json:"company_strategy"`
}

// ES添削の指示遵守の許容範囲。
//
// プロンプトは feedback を「400字程度」、improved_text を「元の文字数の
// 110〜130%を目安」と指示している。「程度」「目安」なので幅を持たせるが、
// 幅の根拠をここに書いておく。数字を動かすと遵守率も動くため、
// 実行間で比較するには固定されていることが前提。
const (
	esFeedbackMinChars = 250 // 400字の約6割。これ未満は助言として薄い
	esFeedbackMaxChars = 700 // 400字の約1.75倍。これ以上は読まれない
	esImprovedMinRatio = 1.00
	esImprovedMaxRatio = 1.45
)

func newESTarget(modelOverride string) (Target, error) {
	base := strings.TrimSpace(os.Getenv("RAG_REVIEW_URL"))
	if base == "" {
		return nil, fmt.Errorf("ES添削の評価には RAG_REVIEW_URL が必要です（make rag-up で起動し、RAG_REVIEW_URL=http://localhost:9000 を設定）")
	}
	base = strings.TrimRight(base, "/")
	t := &esTarget{baseURL: base, model: strings.TrimSpace(modelOverride)}
	if t.model == "" {
		// モデル名は RAG コンテナの OPENAI_CHAT_MODEL で決まるため、
		// /health から実際の値を取る。取れないまま推測で埋めると、
		// 結果に残るモデル名が嘘になりコスト・比較の前提が崩れる。
		m, err := fetchRAGChatModel(base)
		if err != nil {
			return nil, fmt.Errorf("RAG のモデル名を取得できませんでした（-model で明示するか RAG を起動してください）: %w", err)
		}
		t.model = m
	}
	return t, nil
}

// fetchRAGChatModel は RAG の /health から ES添削に使われるモデル名を取る。
func fetchRAGChatModel(baseURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return "", err
	}
	ragclient.SetAuthHeader(req)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var parsed struct {
		ChatModel string `json:"chat_model"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if strings.TrimSpace(parsed.ChatModel) == "" {
		return "", fmt.Errorf("/health に chat_model がありません（RAG が古い可能性）")
	}
	return parsed.ChatModel, nil
}

func (t *esTarget) Name() string     { return TargetES }
func (t *esTarget) Model() string    { return t.model }
func (t *esTarget) Endpoint() string { return t.baseURL + "/es/review" }

// EstimateTokens は事前のコスト概算。
//
// ES添削は #1521 で2回の呼び出しに分割されている（評価＋改善文）。
// 入力はどちらもES本文を含むので2倍、出力は評価の約900字と改善文の約130%で見る。
func (t *esTarget) EstimateTokens(c Case) (int, int) {
	es := ApproxTokensJA(c.Input.ESText)
	// 600 はプロンプト定型文ぶん（2回合計）の概算。
	return es*2 + 600, 900 + int(float64(es)*1.3)
}

func (t *esTarget) Run(ctx context.Context, c Case) Observation {
	o := newObservation(c)
	payload := map[string]string{
		"es_text":       c.Input.ESText,
		"question_type": c.Input.QuestionType,
		"company_name":  c.Input.CompanyName,
	}
	header := map[string]string{}
	if tok := strings.TrimSpace(os.Getenv("RAG_INTERNAL_TOKEN")); tok != "" {
		header[ragclient.InternalTokenHeader] = tok
	}
	start := time.Now()
	status, body, err := postJSON(ctx, t.Endpoint(), payload, header)
	o.LatencyMS = time.Since(start).Milliseconds()

	// トークン数は RAG のレスポンスに含まれないため概算になる。
	o.TokensEstimated = true
	o.PromptTokens, o.CompletionTokens = t.EstimateTokens(c)

	if err != nil {
		// RAG へ到達できていない。品質ではないので計測対象から外す。
		o.Broken, o.BrokenReason, o.Error = true, BrokenNetwork, err.Error()
		return o
	}
	return evaluateESResponse(o, c, status, body)
}

// evaluateESResponse は /es/review のレスポンスを指標へ落とす（純粋関数）。
//
// ステータスコードで破損理由を分けている。es_review.py は出力上限に到達した
// ときだけ 422 を返し（再試行しても上限に届いた場合）、それ以外の失敗は 500 を返す。
func evaluateESResponse(o Observation, c Case, status int, body []byte) Observation {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		// RAG_INTERNAL_TOKEN の未設定・不一致。品質ではなく設定の問題なので
		// 破損率に混ぜず、実行を打ち切らせる。
		o.Broken, o.BrokenReason, o.Fatal = true, BrokenCallFailed, true
		o.Error = fmt.Sprintf("RAGが認証を拒否しました（HTTP %d）。RAG_INTERNAL_TOKEN を確認してください", status)
		return o
	case status == http.StatusUnprocessableEntity:
		o.Broken, o.BrokenReason, o.Error = true, BrokenTruncated, snippet(body)
		return o
	case status != http.StatusOK:
		reason := BrokenCallFailed
		// 500 の detail に JSON 解析の失敗が出ていれば JSON不正として数える。
		if strings.Contains(strings.ToLower(string(body)), "json") {
			reason = BrokenJSONInvalid
		}
		o.Broken, o.BrokenReason, o.Error = true, reason, snippet(body)
		return o
	}

	var resp esReviewResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		o.Broken, o.BrokenReason, o.Error = true, BrokenJSONInvalid, err.Error()
		return o
	}
	if resp.SpecificityScore == nil || resp.StarScore == nil || resp.LengthBalanceScore == nil {
		o.Broken, o.BrokenReason, o.Error = true, BrokenSchema, "スコアのキーが欠けている"
		return o
	}
	if strings.TrimSpace(resp.Feedback) == "" || strings.TrimSpace(resp.ImprovedText) == "" {
		o.Broken, o.BrokenReason, o.Error = true, BrokenSchema, "feedback または improved_text が空"
		return o
	}

	// スコアは3軸の平均。company_fit_score は企業情報が無いと null になるので混ぜない。
	raw := float64(*resp.SpecificityScore+*resp.StarScore+*resp.LengthBalanceScore) / 3
	o.RawScore = raw
	o.Score = (raw - 1) / 9 // 1〜10 を 0〜1 へ

	if n := runeLen(resp.Feedback); n < esFeedbackMinChars || n > esFeedbackMaxChars {
		o.Violations = append(o.Violations, "feedback_length")
	}
	if src := runeLen(c.Input.ESText); src > 0 {
		ratio := float64(runeLen(resp.ImprovedText)) / float64(src)
		if ratio < esImprovedMinRatio || ratio > esImprovedMaxRatio {
			o.Violations = append(o.Violations, "improved_text_ratio")
		}
	}
	// 企業情報が無いときに企業評価を出さない（#1524）。
	if strings.TrimSpace(c.Input.CompanyName) == "" {
		if resp.CompanyFitScore != nil {
			o.Violations = append(o.Violations, "company_fit_without_context")
		}
		if resp.CompanyStrategy != nil && strings.TrimSpace(*resp.CompanyStrategy) != "" {
			o.Violations = append(o.Violations, "company_strategy_without_context")
		}
	}
	return o
}
