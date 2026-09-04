package hr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"Backend/internal/ragclient"
)

// ErrSemanticSearchUnavailable はRAGサービス未設定・不通のとき（503で返す）。
var ErrSemanticSearchUnavailable = errors.New("semantic search unavailable")

const semanticSearchTimeout = 20 * time.Second

type semanticQueryRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k"`
}

type semanticQueryHit struct {
	UserID uint    `json:"user_id"`
	Score  float64 `json:"score"`
}

type semanticQueryResponse struct {
	Hits []semanticQueryHit `json:"hits"`
}

type semanticIndexRequest struct {
	UserID uint   `json:"user_id"`
	Text   string `json:"text"`
}

// StudentSemanticClient は RAG の学生プロフィール検索エンドポイントを呼ぶ（#1094）。
type StudentSemanticClient struct {
	client *http.Client
}

func NewStudentSemanticClient() *StudentSemanticClient {
	return &StudentSemanticClient{client: &http.Client{Timeout: semanticSearchTimeout}}
}

func ragBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("RAG_REVIEW_URL")), "/")
}

func (c *StudentSemanticClient) do(ctx context.Context, method, path string, payload any, out any) error {
	base := ragBaseURL()
	if base == "" {
		return ErrSemanticSearchUnavailable
	}
	var body *bytes.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	} else {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	ragclient.SetAuthHeader(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSemanticSearchUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrSemanticSearchUnavailable, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Query は自然文クエリに意味的に近い学生IDを関連度順で返す。
func (c *StudentSemanticClient) Query(ctx context.Context, query string, topK int) ([]uint, error) {
	var res semanticQueryResponse
	if err := c.do(ctx, http.MethodPost, "/student-search/query",
		semanticQueryRequest{Query: query, TopK: topK}, &res); err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(res.Hits))
	for _, h := range res.Hits {
		if h.UserID > 0 {
			ids = append(ids, h.UserID)
		}
	}
	return ids, nil
}

// Index は学生プロフィールをベクトル化して登録・更新する。
func (c *StudentSemanticClient) Index(ctx context.Context, userID uint, text string) error {
	return c.do(ctx, http.MethodPost, "/student-search/index",
		semanticIndexRequest{UserID: userID, Text: text}, nil)
}

// Delete は学生のベクトルを削除する（公開同意の撤回・退会時）。
func (c *StudentSemanticClient) Delete(ctx context.Context, userID uint) error {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/student-search/%d", userID), nil, nil)
}
