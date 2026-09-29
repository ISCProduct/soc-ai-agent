package es

import (
	"Backend/internal/ragclient"
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

	// リクエストIDを RAG まで伝播させるため ctx 付きで作る(#1188)
	ragReq, err := http.NewRequestWithContext(ctx.Request().Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create RAG request")
	}
	ragReq.Header.Set("Content-Type", "application/json")
	ragclient.SetAuthHeader(ragReq)

	// RAG側は評価と改善文で2回OpenAIを直列呼び出しし、各段が出力上限到達時に1回
	// 再試行するため、LLM呼び出しは最悪4回直列になる(#1521)。1回あたり最大
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
