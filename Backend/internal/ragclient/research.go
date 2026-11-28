package ragclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 企業研究AIの文章の登録・検索（SOCAIAGENT-725）。RAG 側は rag/routers/company_research.py。

// Usage は RAG が返すベクトル化のトークン量。Backend が api_call_logs に記録する。
type Usage struct {
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	Calls            int    `json:"calls"`
}

// ResearchDocument は登録する文章1つ。
type ResearchDocument struct {
	DocumentID uint   `json:"document_id"`
	SourceType string `json:"source_type"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	AsOf       string `json:"as_of,omitempty"` // YYYY-MM-DD
	Body       string `json:"body"`
}

// ResearchHit は検索で見つかったチャンク1つ。
type ResearchHit struct {
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
	CompanyID  uint    `json:"company_id"`
	DocumentID uint    `json:"document_id"`
	SourceType string  `json:"source_type"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	AsOf       string  `json:"as_of"`
}

// ResearchClient は RAG の企業研究エンドポイントを呼ぶ。baseURL が空なら呼ばない。
type ResearchClient struct {
	baseURL string
	http    *http.Client
}

func NewResearchClient(baseURL string) *ResearchClient {
	return &ResearchClient{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 60 * time.Second}}
}

func (c *ResearchClient) Enabled() bool { return c != nil && c.baseURL != "" }

// Index は1社分の文章を登録する（同じ document_id のチャンクは置き換わる）。
func (c *ResearchClient) Index(ctx context.Context, companyID uint, docs []ResearchDocument) (Usage, error) {
	var resp struct {
		Usage Usage `json:"usage"`
	}
	err := c.post(ctx, "/company-research/index", map[string]any{"company_id": companyID, "documents": docs}, &resp)
	return resp.Usage, err
}

// Query は企業IDで絞って、質問に近いチャンクを返す。
func (c *ResearchClient) Query(ctx context.Context, companyIDs []uint, question string, topK int) ([]ResearchHit, Usage, error) {
	var resp struct {
		Hits  []ResearchHit `json:"hits"`
		Usage Usage         `json:"usage"`
	}
	err := c.post(ctx, "/company-research/query", map[string]any{"company_ids": companyIDs, "question": question, "top_k": topK}, &resp)
	return resp.Hits, resp.Usage, err
}

func (c *ResearchClient) post(ctx context.Context, path string, payload any, out any) error {
	if !c.Enabled() {
		return fmt.Errorf("ragclient: RAG_REVIEW_URL が設定されていません")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	SetAuthHeader(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ragclient: %s: %w", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ragclient: %s がステータス %d を返しました", path, resp.StatusCode)
	}
	return json.Unmarshal(raw, out)
}
