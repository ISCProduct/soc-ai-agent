package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// AdminVectorController は RAG ベクトルDB管理 API のプロキシ。
type AdminVectorController struct {
	ragBaseURL string
	httpClient *http.Client
}

func NewAdminVectorController() *AdminVectorController {
	base := strings.TrimSpace(os.Getenv("RAG_REVIEW_URL"))
	return &AdminVectorController{
		ragBaseURL: strings.TrimRight(base, "/"),
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// Status handles GET /api/admin/vector/status?company=
func (c *AdminVectorController) Status(ctx echo.Context) error {
	if c.ragBaseURL == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "RAG_REVIEW_URL is not configured")
	}
	endpoint := c.ragBaseURL + "/vector/status"
	if company := strings.TrimSpace(ctx.QueryParam("company")); company != "" {
		endpoint += "?company=" + url.QueryEscape(company)
	}
	return c.proxyJSON(ctx, http.MethodGet, endpoint, nil)
}

// Reembed handles POST /api/admin/vector/reembed
func (c *AdminVectorController) Reembed(ctx echo.Context) error {
	if c.ragBaseURL == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "RAG_REVIEW_URL is not configured")
	}
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	return c.proxyJSON(ctx, http.MethodPost, c.ragBaseURL+"/vector/reembed", body)
}

// Stats handles GET /api/admin/vector/stats - RAG利用統計情報の取得
func (c *AdminVectorController) Stats(ctx echo.Context) error {
	if c.ragBaseURL == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "RAG_REVIEW_URL is not configured")
	}
	return c.proxyJSON(ctx, http.MethodGet, c.ragBaseURL+"/vector/stats", nil)
}

// Collections handles GET /api/admin/vector/collections - RAG全コレクション情報の取得
func (c *AdminVectorController) Collections(ctx echo.Context) error {
	if c.ragBaseURL == "" {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "RAG_REVIEW_URL is not configured")
	}
	return c.proxyJSON(ctx, http.MethodGet, c.ragBaseURL+"/vector/collections", nil)
}

func (c *AdminVectorController) proxyJSON(ctx echo.Context, method, endpoint string, body []byte) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx.Request().Context(), method, endpoint, reader)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, fmt.Sprintf("RAG request failed: %v", err))
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "failed to read RAG response")
	}

	var payload any
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &payload); err != nil {
			payload = map[string]any{"raw": string(respBody)}
		}
	} else {
		payload = map[string]any{}
	}
	return ctx.JSON(resp.StatusCode, payload)
}
