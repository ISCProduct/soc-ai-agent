package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"Backend/internal/ragclient"
)

// AdminVectorService は RAG ベクトルDB管理 API へのアクセスを担う。
type AdminVectorService struct {
	ragBaseURL string
	httpClient *http.Client
}

func NewAdminVectorService() *AdminVectorService {
	base := strings.TrimSpace(os.Getenv("RAG_REVIEW_URL"))
	return &AdminVectorService{
		ragBaseURL: strings.TrimRight(base, "/"),
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (s *AdminVectorService) Configured() bool {
	return s.ragBaseURL != ""
}

type AdminVectorProxyResult struct {
	StatusCode int
	Payload    any
}

func (s *AdminVectorService) Status(ctx context.Context, company string) (*AdminVectorProxyResult, error) {
	endpoint := s.ragBaseURL + "/vector/status"
	if company = strings.TrimSpace(company); company != "" {
		endpoint += "?company=" + url.QueryEscape(company)
	}
	return s.proxyJSON(ctx, http.MethodGet, endpoint, nil)
}

func (s *AdminVectorService) Reembed(ctx context.Context, body []byte) (*AdminVectorProxyResult, error) {
	return s.proxyJSON(ctx, http.MethodPost, s.ragBaseURL+"/vector/reembed", body)
}

func (s *AdminVectorService) Stats(ctx context.Context) (*AdminVectorProxyResult, error) {
	return s.proxyJSON(ctx, http.MethodGet, s.ragBaseURL+"/vector/stats", nil)
}

func (s *AdminVectorService) Collections(ctx context.Context) (*AdminVectorProxyResult, error) {
	return s.proxyJSON(ctx, http.MethodGet, s.ragBaseURL+"/vector/collections", nil)
}

func (s *AdminVectorService) proxyJSON(ctx context.Context, method, endpoint string, body []byte) (*AdminVectorProxyResult, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ragclient.SetAuthHeader(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RAG request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read RAG response: %w", err)
	}

	var payload any
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &payload); err != nil {
			payload = map[string]any{"raw": string(respBody)}
		}
	} else {
		payload = map[string]any{}
	}
	return &AdminVectorProxyResult{StatusCode: resp.StatusCode, Payload: payload}, nil
}
