package es

import (
	"Backend/internal/middleware"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/ragclient"
	"Backend/internal/services/esreview"
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
	// reviews は添削結果の永続化(#1531)。未注入なら保存しない（従来どおり転送だけ）。
	reviews *esreview.Service
}

func NewESReviewController(openaiClient *openai.Client) *ESReviewController {
	return &ESReviewController{openaiClient: openaiClient}
}

// SetReviewService は添削結果の永続化を注入する(#1531、オプション)。
func (c *ESReviewController) SetReviewService(s *esreview.Service) {
	c.reviews = s
}

type esReviewRequest struct {
	ESText string `json:"es_text"`
	// Question は利用者が入力した設問文(#1531)。同一設問の前回結果と突き合わせるのに使う。
	// 種別だけだと「学チカ」と「ガクチカ」が別扱いになり差分が出ない。未指定でも添削はできる。
	Question     string `json:"question"`
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
	ESText               string `json:"es_text"`
	QuestionType         string `json:"question_type"`
	CompanyName          string `json:"company_name,omitempty"`
	TechStack            string `json:"tech_stack,omitempty"`
	CharLimit            *int   `json:"char_limit,omitempty"`
	CharLimitMode        string `json:"char_limit_mode,omitempty"`
	IncludeFeedbackItems bool   `json:"include_feedback_items"`
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
	// 永続化に使う評価項目(#1531)。
	//
	// 必須の3項目もポインタで受ける。値型だと項目が欠けた応答で 0 になり、
	// 丸めて 1（最低評価）として履歴とスコアに流れる。欠落を最低評価として
	// 記録するのは「基準の無い減点」で、履歴書が #1529 で明示的に避けた判断と逆向き。
	SpecificityScore   *int   `json:"specificity_score"`
	StarScore          *int   `json:"star_score"`
	LengthBalanceScore *int   `json:"length_balance_score"`
	CompanyFitScore    *int   `json:"company_fit_score"`
	Feedback           string `json:"feedback"`
}

// Review POST /api/es/review
func (c *ESReviewController) Review(ctx echo.Context) error {
	var req esReviewRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	// 形式チェックの位置は、利用者が送った原文（trim 前）を基準にする（#1530）
	rawText := req.ESText
	req.ESText = strings.TrimSpace(req.ESText)
	if req.ESText == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "es_text is required")
	}
	if req.QuestionType == "" {
		req.QuestionType = "その他"
	}

	log.Printf("es_review: rag request question_type=%q company=%q char_limit=%v", req.QuestionType, req.CompanyName, req.CharLimit)
	status, respBody, err := postESReview(ctx, esReviewRAGRequest{
		ESText:               req.ESText,
		QuestionType:         req.QuestionType,
		CompanyName:          req.CompanyName,
		CharLimit:            req.CharLimit,
		CharLimitMode:        req.CharLimitMode,
		IncludeFeedbackItems: true,
	}, c.openaiClient, usagectx.FeatureESReview)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return ctx.JSONBlob(status, respBody)
	}

	// RAG の応答（usage は取り除き済み）に、原文の形式チェック結果を足す（#1530）
	body, err := withFormatFindings(respBody, formatFindings(rawText, req.CharLimit, req.CharLimitMode))
	if err != nil {
		// 形式チェックは付加機能なので、RAG の応答が想定外の形でも添削結果はそのまま返す
		log.Printf("es_review: format_findings を付けられないため RAG の応答を素通しする: %v", err)
		body = respBody
	}
	// 添削が成功したときだけ履歴に残す(#1531)。失敗した応答まで残すと、
	// 前回との差分が「失敗した回」と比較されて意味を失う。
	return ctx.JSONBlob(status, c.persist(ctx, req, body))
}

// persist は添削結果を保存し、同一設問の前回結果を previous として本文へ足す。
//
// 保存できなくても添削そのものは成立しているので、本文はそのまま返す。
// 履歴が残らないことより、添削が使えなくなることの方が重い。
func (c *ESReviewController) persist(ctx echo.Context, req esReviewRequest, body []byte) []byte {
	if c.reviews == nil {
		return body
	}
	// 未ログインでも使える経路なので、実行者が取れないときは保存しない(#1531)。
	userID, ok := middleware.UserIDFromContext(ctx.Request().Context())
	if !ok || userID == 0 {
		return body
	}
	var parsed esReviewRAGResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Printf("[ESReview] RAG応答を解釈できず保存をスキップ user=%d: %v", userID, err)
		return body
	}
	// 組織が確定しないまま保存すると、既定組織の行として永続化され、
	// 教員向けクエリの学園一致条件に永久に当たらなくなる（誰も気づけない）。
	// 添削そのものは返すので、保存だけ諦める（#1531）。
	orgID, ok := middleware.OrganizationIDFromContext(ctx.Request().Context())
	if !ok || orgID == 0 {
		log.Printf("[ESReview] 組織が解決できないため保存をスキップ user=%d", userID)
		return body
	}
	// 評価が1つでも欠けた応答は履歴に残さない。差分もスコア連携も評価を前提にしており、
	// 欠けた行を残すと「最低評価の回」として後から比較されてしまう。
	if parsed.SpecificityScore == nil || parsed.StarScore == nil || parsed.LengthBalanceScore == nil {
		log.Printf("[ESReview] RAG応答に評価が欠けているため保存をスキップ user=%d", userID)
		return body
	}

	var charLimit *uint
	if req.CharLimit != nil && *req.CharLimit > 0 {
		v := uint(*req.CharLimit)
		charLimit = &v
	}

	_, previous, err := c.reviews.Save(esreview.Input{
		UserID:             userID,
		OrganizationID:     orgID,
		Question:           req.Question,
		QuestionType:       req.QuestionType,
		CompanyName:        req.CompanyName,
		ESText:             req.ESText,
		CharLimit:          charLimit,
		CharLimitMode:      req.CharLimitMode,
		SpecificityScore:   *parsed.SpecificityScore,
		StarScore:          *parsed.StarScore,
		LengthBalanceScore: *parsed.LengthBalanceScore,
		CompanyFitScore:    parsed.CompanyFitScore,
		Feedback:           parsed.Feedback,
		ImprovedText:       parsed.ImprovedText,
	})
	if err != nil {
		log.Printf("[ESReview] 保存に失敗 user=%d: %v", userID, err)
		return body
	}
	if previous == nil {
		return body
	}
	return withPrevious(body, previous)
}

// esReviewPrevious は前回の添削のうち差分表示に使う項目だけ(#1531)。
//
// models.ESReview をそのまま返すと ES本文・改善文・フィードバック全文まで
// レスポンスに乗る。本人のデータなので漏洩ではないが、画面が使うのは
// 日付とスコアだけで、無駄に重い。
type esReviewPrevious struct {
	ID                 uint   `json:"id"`
	CreatedAt          string `json:"created_at"`
	Question           string `json:"question"`
	QuestionType       string `json:"question_type"`
	SpecificityScore   uint8  `json:"specificity_score"`
	StarScore          uint8  `json:"star_score"`
	LengthBalanceScore uint8  `json:"length_balance_score"`
	CompanyFitScore    *uint8 `json:"company_fit_score"`
}

// withPrevious は本文に previous を足す。解釈できない本文は触らずに返す
// （表示を壊さないことを優先し、差分だけ諦める）。
func withPrevious(body []byte, previous *models.ESReview) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	encoded, err := json.Marshal(esReviewPrevious{
		ID:                 previous.ID,
		CreatedAt:          previous.CreatedAt.Format(time.RFC3339),
		Question:           previous.Question,
		QuestionType:       previous.QuestionType,
		SpecificityScore:   previous.SpecificityScore,
		StarScore:          previous.StarScore,
		LengthBalanceScore: previous.LengthBalanceScore,
		CompanyFitScore:    previous.CompanyFitScore,
	})
	if err != nil {
		return body
	}
	raw["previous"] = encoded
	merged, err := json.Marshal(raw)
	if err != nil {
		return body
	}
	return merged
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
	// 手前の経路(CloudFront origin_read_timeout / ALB idle_timeout / stagingのedge nginx
	// proxy_read_timeout)は #1556 で90秒へ揃えた。ここを上流より1段短い85秒にするのは、
	// 利用者向けの文面が変わるからではない。prodのCloudFrontは502/504とも503+
	// /service-unavailable.html へ、stagingのnginxも502/504とも「起動中」ページへ差し替える
	// ため、85で切っても90で切っても画面は同じ。目的は上流の汎用504より先に手放して、
	// このログと ALB アクセスログ(target_status_code=502 / target_processing_time)に
	// 原因を残すこと。
	// 90秒は「待つ上限」であって「処理の上限」ではない。各段が上限近くまで粘るケースは
	// 救えないので、RAG側の経過時間ガード(#1523)と併用する前提。
	client := &http.Client{Timeout: 85 * time.Second}
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
