package controllers

import (
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

type ESReviewController struct{}

func NewESReviewController() *ESReviewController {
	return &ESReviewController{}
}

type esReviewRequest struct {
	ESText       string `json:"es_text"`
	QuestionType string `json:"question_type"`
	CompanyName  string `json:"company_name"`
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

	ragURL := strings.TrimSpace(os.Getenv("RAG_REVIEW_URL"))
	if ragURL == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "RAG_REVIEW_URL is not configured")
	}

	// RAGサービスへのリクエストボディを構築
	body, err := json.Marshal(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to encode request")
	}

	url := strings.TrimRight(ragURL, "/") + "/es/review"
	log.Printf("es_review: rag request question_type=%q company=%q", req.QuestionType, req.CompanyName)

	ragReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create RAG request")
	}
	ragReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(ragReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "RAG service unavailable: "+err.Error())
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to read RAG response")
	}

	// RAGからのレスポンスをそのままクライアントへ転送
	return ctx.JSONBlob(resp.StatusCode, respBody)
}
