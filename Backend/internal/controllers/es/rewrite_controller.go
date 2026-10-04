package es

import (
	"Backend/internal/openai"
	"Backend/internal/usagectx"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type ESRewriteController struct {
	// openaiClient は生成には使わない。RAG が実行した生成のトークン使用量を
	// api_call_logs へ記録するためだけに持つ(#1533)。nil でも動く。
	openaiClient *openai.Client
}

func NewESRewriteController(openaiClient *openai.Client) *ESRewriteController {
	return &ESRewriteController{openaiClient: openaiClient}
}

type esRewriteRequest struct {
	OriginalText string `json:"original_text"`
	QuestionType string `json:"question_type"` // "志望動機" | "自己PR" | "学チカ" | "その他"
	TechStack    string `json:"tech_stack"`    // 任意: 使用技術スタック
	CompanyName  string `json:"company_name"`  // 任意: 志望企業名（RAG が企業情報を参照する）
	// 任意: 設問の文字数上限(#1523)
	CharLimit     *int   `json:"char_limit"`
	CharLimitMode string `json:"char_limit_mode"`
}

type starBreakdown struct {
	Situation string `json:"situation"`
	Task      string `json:"task"`
	Action    string `json:"action"`
	Result    string `json:"result"`
}

type esRewriteResponse struct {
	RewrittenText string        `json:"rewritten_text"`
	Star          starBreakdown `json:"star"`
	// 字数の結果(#1523)。既存キーは変えず追加だけしている
	ImprovedTextLength int   `json:"improved_text_length"`
	CharLimitSatisfied *bool `json:"char_limit_satisfied"`
}

// Rewrite POST /api/es/rewrite
//
// 生成は RAG の /es/review に一本化してある(#1533)。ここでプロンプトは組まない。
// 以前は Backend が独自プロンプト（gpt-4o-mini・インジェクション対策なし・字数指示
// 120〜150%）で呼んでいたため、同じESでもES添削タブと違う書き換え案が返っていた。
// レスポンスは従来の { rewritten_text, star } を保ったまま返す。
func (c *ESRewriteController) Rewrite(ctx echo.Context) error {
	var req esRewriteRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	req.OriginalText = strings.TrimSpace(req.OriginalText)
	if req.OriginalText == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "original_text is required")
	}
	if req.QuestionType == "" {
		req.QuestionType = "その他"
	}

	// 未ログイン利用が仕様の経路なので主体は載らないことがある。機能名だけ付けて
	// 費用を機能別に割る（#1294）。es_rewrite と es_review の内訳はここで分かれる。
	status, body, err := postESReview(ctx, esReviewRAGRequest{
		ESText:        req.OriginalText,
		QuestionType:  req.QuestionType,
		CompanyName:   req.CompanyName,
		TechStack:     req.TechStack,
		CharLimit:     req.CharLimit,
		CharLimitMode: req.CharLimitMode,
	}, c.openaiClient, usagectx.FeatureESRewrite)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		// RAG の案内文（422「文章が長すぎて…」など）をそのまま利用者へ渡す
		return ctx.JSONBlob(status, body)
	}

	var rag esReviewRAGResponse
	if err := json.Unmarshal(body, &rag); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to parse RAG response")
	}

	return ctx.JSON(http.StatusOK, esRewriteResponse{
		RewrittenText:      rag.ImprovedText,
		Star:               rag.Star,
		ImprovedTextLength: rag.ImprovedTextLength,
		CharLimitSatisfied: rag.CharLimitSatisfied,
	})
}
