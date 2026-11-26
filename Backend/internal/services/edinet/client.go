// Package edinet は金融庁 EDINET API v2 から有価証券報告書を取得する（SOCAIAGENT-726）。
//
// 仕様: EDINET API 仕様書（ESE140206.pdf）。使うのは次の2つだけ。
//   - 書類一覧 API: GET /documents.json?date=YYYY-MM-DD&type=2 … その日に提出された書類の一覧
//   - 書類取得 API: GET /documents/{docID}?type=5           … XBRL を CSV に変換した ZIP
//
// API キーは Subscription-Key クエリで渡す。キーはログやエラー文に出さない。
// 短時間に大量に呼ぶと 429 になるため、呼び出しの間隔を空ける（minInterval）。
package edinet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ErrRateLimited は 429（短時間に大量のリクエスト）を受けたことを表す。呼び出し側はその日の処理を打ち切る。
var ErrRateLimited = errors.New("edinet: rate limited (429)")

const minInterval = 1200 * time.Millisecond

// Document は書類一覧 API の1件（使う項目だけ）。
type Document struct {
	DocID            string `json:"docID"`
	EdinetCode       string `json:"edinetCode"`
	SecCode          string `json:"secCode"`
	JCN              string `json:"JCN"` // 提出者の法人番号
	FilerName        string `json:"filerName"`
	OrdinanceCode    string `json:"ordinanceCode"`
	FormCode         string `json:"formCode"`
	DocTypeCode      string `json:"docTypeCode"`
	PeriodEnd        string `json:"periodEnd"`
	SubmitDateTime   string `json:"submitDateTime"`
	ParentDocID      string `json:"parentDocID"`
	WithdrawalStatus string `json:"withdrawalStatus"`
	CSVFlag          string `json:"csvFlag"`
}

// IsAnnualReport は一般企業の有価証券報告書（訂正を含む）で、CSV が取れるものかを返す。
// 投資信託など（府令コード 030）は対象外。取り下げられた書類も除く。
func (d Document) IsAnnualReport() bool {
	return (d.DocTypeCode == "120" || d.DocTypeCode == "130") &&
		d.OrdinanceCode == "010" && d.WithdrawalStatus == "0" && d.CSVFlag == "1" && d.JCN != ""
}

// ViewerURL は EDINET の書類閲覧ページの URL（学生に見せる出典）。
func (d Document) ViewerURL() string {
	return "https://disclosure2.edinet-fsa.go.jp/WZEK0040.aspx?" + url.QueryEscape(d.DocID)
}

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client

	mu   sync.Mutex
	last time.Time
}

func NewClient(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = "https://api.edinet-fsa.go.jp/api/v2"
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: &http.Client{Timeout: 60 * time.Second}}
}

// Enabled はキーが設定されているかを返す。キーが無ければ取り込みを行わない。
func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

// ListDocuments は date に提出された書類の一覧を返す。
func (c *Client) ListDocuments(ctx context.Context, date time.Time) ([]Document, error) {
	q := url.Values{"date": {date.Format("2006-01-02")}, "type": {"2"}}
	body, err := c.get(ctx, "/documents.json", q)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Metadata struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"metadata"`
		Results []Document `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("edinet: 書類一覧の解析に失敗: %w", err)
	}
	// HTTP 200 でも metadata.status にエラーが入ることがある。
	if resp.Metadata.Status != "" && resp.Metadata.Status != "200" {
		return nil, fmt.Errorf("edinet: 書類一覧がエラーを返しました: status=%s %s", resp.Metadata.Status, resp.Metadata.Message)
	}
	return resp.Results, nil
}

// FetchCSVZip は書類の CSV（ZIP）を返す。
func (c *Client) FetchCSVZip(ctx context.Context, docID string) ([]byte, error) {
	return c.get(ctx, "/documents/"+url.PathEscape(docID), url.Values{"type": {"5"}})
}

func (c *Client) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	if !c.Enabled() {
		return nil, errors.New("edinet: API キーが設定されていません")
	}
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	q.Set("Subscription-Key", c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+q.Encode(), nil)
	if err != nil {
		// URL の解析エラーには URL（＝キー）が含まれるので、元のエラー文は返さない。
		return nil, fmt.Errorf("edinet: %s のリクエストを作れません（EDINET_BASE_URL を確認）", path)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		// URL にキーが含まれるので、エラー文からは URL を落とす。
		var uerr *url.Error
		if errors.As(err, &uerr) {
			return nil, fmt.Errorf("edinet: %s の取得に失敗: %w", path, uerr.Err)
		}
		return nil, fmt.Errorf("edinet: %s の取得に失敗", path)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("edinet: %s がステータス %d を返しました", path, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, err
	}
	// 認証エラーなどは HTTP 200 のまま {"StatusCode":401,"message":"..."} の JSON で返る（仕様書 3-3）。
	// 書類一覧なら空の一覧、書類取得なら壊れた ZIP と取り違えないよう、ここでエラーにする。
	if len(body) > 0 && body[0] == '{' {
		var e struct {
			StatusCode int    `json:"StatusCode"`
			Message    string `json:"message"`
		}
		if json.Unmarshal(body, &e) == nil && e.StatusCode != 0 && e.StatusCode != http.StatusOK {
			if e.StatusCode == http.StatusTooManyRequests {
				return nil, ErrRateLimited
			}
			return nil, fmt.Errorf("edinet: %s がエラーを返しました: StatusCode=%d %s", path, e.StatusCode, e.Message)
		}
	}
	return body, nil
}

// wait は前回の呼び出しから minInterval 空ける。
func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d := minInterval - time.Since(c.last); d > 0 && !c.last.IsZero() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
	c.last = time.Now()
	return nil
}
