package es

import (
	"Backend/internal/openai"
	"Backend/internal/ragclient"
	"Backend/internal/usagectx"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type ESReviewController struct {
	// openaiClient は生成には使わない。RAG が実行した生成のトークン使用量を
	// api_call_logs へ記録するためだけに持つ(#1533)。nil でも動く。
	openaiClient *openai.Client
}

func NewESReviewController(openaiClient *openai.Client) *ESReviewController {
	return &ESReviewController{openaiClient: openaiClient}
}

type esReviewRequest struct {
	ESText       string `json:"es_text"`
	QuestionType string `json:"question_type"`
	CompanyName  string `json:"company_name"`
	// CharLimit / CharLimitMode は設問の文字数上限(#1523)。
	// 未指定は RAG 側の既定（上限なし / within）に任せるため omitempty で落とす。
	CharLimit     *int   `json:"char_limit,omitempty"`
	CharLimitMode string `json:"char_limit_mode,omitempty"`
}

// esReviewRAGRequest は RAG の /es/review へ送る本文。
// ES添削・ESリライトの両タブが同じ経路・同じプロンプトを使う(#1533)。
type esReviewRAGRequest struct {
	ESText        string `json:"es_text"`
	QuestionType  string `json:"question_type"`
	CompanyName   string `json:"company_name,omitempty"`
	TechStack     string `json:"tech_stack,omitempty"`
	CharLimit     *int   `json:"char_limit,omitempty"`
	CharLimitMode string `json:"char_limit_mode,omitempty"`
}

// esReviewRAGUsage は RAG が返すトークン使用量（公開APIへは出さない / #1533）。
type esReviewRAGUsage struct {
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	Calls            int    `json:"calls"`
}

// esReviewRAGResponse は RAG の /es/review レスポンスのうち Go が読む項目。
type esReviewRAGResponse struct {
	ImprovedText       string            `json:"improved_text"`
	ImprovedTextLength int               `json:"improved_text_length"`
	CharLimitSatisfied *bool             `json:"char_limit_satisfied"`
	Star               starBreakdown     `json:"star"`
	Usage              *esReviewRAGUsage `json:"usage"`
}

// Review POST /api/es/review
func (c *ESReviewController) Review(ctx echo.Context) error {
	var req esReviewRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	req.ESText = strings.TrimSpace(req.ESText)
	if req.ESText == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "es_text is required")
	}
	if req.QuestionType == "" {
		req.QuestionType = "その他"
	}

	log.Printf("es_review: rag request question_type=%q company=%q char_limit=%v", req.QuestionType, req.CompanyName, req.CharLimit)
	status, respBody, err := postESReview(ctx, esReviewRAGRequest{
		ESText:        req.ESText,
		QuestionType:  req.QuestionType,
		CompanyName:   req.CompanyName,
		CharLimit:     req.CharLimit,
		CharLimitMode: req.CharLimitMode,
	}, c.openaiClient, usagectx.FeatureESReview)
	if err != nil {
		return err
	}

	// RAGからのレスポンスをそのままクライアントへ転送（usage だけは取り除く）
	return ctx.JSONBlob(status, respBody)
}

// postESReview は RAG の /es/review を呼び、(ステータス, 本文, エラー) を返す。
// 本文からは usage を取り除き、取り除いた使用量は api_call_logs へ記録する(#1533)。
//
// ES添削・ESリライトの唯一の生成経路。プロンプトとインジェクション対策は RAG 側の
// services/es_review.py に一本化してあるので、ここでプロンプトを組み立てないこと。
func postESReview(ctx echo.Context, payload esReviewRAGRequest, cli *openai.Client, feature string) (int, []byte, error) {
	ragURL := strings.TrimSpace(os.Getenv("RAG_REVIEW_URL"))
	if ragURL == "" {
		return 0, nil, echo.NewHTTPError(http.StatusServiceUnavailable, "RAG_REVIEW_URL is not configured")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to encode request")
	}

	url := strings.TrimRight(ragURL, "/") + "/es/review"

	// リクエストIDを RAG まで伝播させるため ctx 付きで作る(#1188)
	ragReq, err := http.NewRequestWithContext(ctx.Request().Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create RAG request")
	}
	ragReq.Header.Set("Content-Type", "application/json")
	ragclient.SetAuthHeader(ragReq)

	// RAG側は評価と改善文で直列にOpenAIを呼ぶ。各段が出力上限到達時に1回再試行し、
	// さらに改善文は字数超過で最大2回作り直すため、LLM呼び出しは最悪8回直列になる
	// （評価2 + 改善文 3回 × 各1回再試行 / #1521, #1523）。1回あたり最大
	// RAG_OPENAI_TIMEOUT_SEC(既定60秒)で、さらに企業名指定時は前段のWeb Searchも直列。
	// 60秒だとRAGが正常に生成中でもBackendが先に打ち切り、422の案内文も結果も
	// ユーザーへ届かないため延長する。
	// 注意: 現状この180秒は最後まで効かない。手前のALB(idle_timeout未指定=既定60秒)と
	// CloudFront(origin_read_timeout=60秒)が先に切るため、実効値は60秒。
	// インフラ側の延長は #1556 で対応する。
	client := &http.Client{Timeout: 180 * time.Second}
	started := time.Now()
	resp, err := client.Do(ragReq)
	if err != nil {
		return 0, nil, echo.NewHTTPError(http.StatusBadGateway, "RAG service unavailable: "+err.Error())
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to read RAG response")
	}
	if resp.StatusCode == http.StatusOK {
		respBody = consumeRAGUsage(ctx, cli, feature, respBody, time.Since(started))
	}
	return resp.StatusCode, respBody, nil
}

// consumeRAGUsage は RAG レスポンスの usage を api_call_logs へ記録し、本文から取り除く。
//
// RAG は自前の記録先を持たないため、機能別コスト（es_review / es_rewrite）の内訳は
// ここでしか残せない。usage は内部情報なので公開APIの本文には残さない。
// 解析できない本文は触らずに返す（表示を壊さないことを優先し、記録だけ諦める）。
func consumeRAGUsage(ctx echo.Context, cli *openai.Client, feature string, body []byte, latency time.Duration) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	usageRaw, ok := raw["usage"]
	if !ok {
		return body
	}
	delete(raw, "usage")
	cleaned, err := json.Marshal(raw)
	if err != nil {
		return body
	}
	var usage esReviewRAGUsage
	if err := json.Unmarshal(usageRaw, &usage); err == nil && usage.Calls > 0 {
		// 機能名は呼び出し元が定数で渡す（#1294 の方針）。実行主体はリクエストの
		// コンテキストから取る（未ログイン利用も仕様の経路なので無ければ NULL）。
		cli.ReportProxyUsage(usagectx.WithFeature(ctx.Request().Context(), feature), openai.ProxyUsage{
			Model:            usage.Model,
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			Latency:          latency,
		})
	}
	return cleaned
}
