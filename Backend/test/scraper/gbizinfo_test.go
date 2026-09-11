package scraper_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Backend/internal/scraper"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gBizINFO APIのモックレスポンスを構築するヘルパー
func newGBizServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *scraper.GBizClient) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := scraper.NewGBizClient(srv.URL, "test-token")
	return srv, client
}

func gbizResponse(records []map[string]any) string {
	b, _ := json.Marshal(map[string]any{"hojin-infos": records})
	return string(b)
}

// ── GBizClient.Search() ───────────────────────────────────────────────────────

func TestGBizClient_Search_Success(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-token", r.Header.Get("X-hojinInfo-api-token"))
		assert.Equal(t, "株式会社テスト", r.URL.Query().Get("name"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse([]map[string]any{
			{
				"corporate_number": "1234567890123",
				"name":             "株式会社テスト",
				"postal_code":      "150-0001",
				"location":         "東京都渋谷区",
				"business_summary": map[string]any{"major_classification_name": "情報通信業"},
				"company_url":      "https://test.example.com",
			},
		})))
	})

	records, err := client.Search(context.Background(), "株式会社テスト", "")
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "1234567890123", records[0].CorporateNumber)
	assert.Equal(t, "株式会社テスト", records[0].Name)
	assert.Equal(t, "東京都渋谷区", records[0].Location)
	assert.Equal(t, "情報通信業", records[0].BusinessSummary.MajorClassificationName)
}

func TestGBizClient_Search_WithPostalCode(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "150-0001", r.URL.Query().Get("postal_code"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse([]map[string]any{
			{"corporate_number": "9999999999999", "name": "テスト株式会社"},
		})))
	})

	records, err := client.Search(context.Background(), "テスト", "150-0001")
	require.NoError(t, err)
	assert.Len(t, records, 1)
}

func TestGBizClient_Search_EmptyResults(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse([]map[string]any{})))
	})

	records, err := client.Search(context.Background(), "存在しない企業名", "")
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestGBizClient_Search_Unauthorized(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := client.Search(context.Background(), "test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401 Unauthorized")
}

func TestGBizClient_Search_ServerError(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.Search(context.Background(), "test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

// ── GBizClient.SearchByKeyword() ─────────────────────────────────────────────

func TestGBizClient_SearchByKeyword_Success(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "IT", r.URL.Query().Get("name"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse([]map[string]any{
			{
				"corporate_number": "1111111111111",
				"name":             "株式会社アルファ",
				"location":         "東京都千代田区",
				"business_summary": map[string]any{"major_classification_name": "情報通信業"},
				"company_url":      "https://alpha.example.com",
			},
			{
				"corporate_number": "2222222222222",
				"name":             "ベータ株式会社",
				"location":         "大阪府大阪市",
				"business_summary": map[string]any{"major_classification_name": "製造業"},
				"company_url":      "https://beta.example.com",
			},
		})))
	})

	nodes, logs, err := client.SearchByKeyword(context.Background(), "IT", 10)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	assert.NotEmpty(t, logs)

	// 1社目の検証
	assert.Equal(t, "1111111111111", nodes[0].CorporateNumber)
	assert.Equal(t, "株式会社アルファ", nodes[0].OfficialName)
	assert.Equal(t, "東京都千代田区", nodes[0].Address)
	assert.Equal(t, "情報通信業", nodes[0].BusinessCategory)
	assert.Equal(t, "https://alpha.example.com", nodes[0].Website)
	assert.Equal(t, 1.0, nodes[0].MatchScore)
	assert.False(t, nodes[0].NeedsReview)

	// 2社目の検証
	assert.Equal(t, "2222222222222", nodes[1].CorporateNumber)
	assert.Equal(t, "ベータ株式会社", nodes[1].OfficialName)
}

func TestGBizClient_SearchByKeyword_EmptyResults(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse([]map[string]any{})))
	})

	nodes, logs, err := client.SearchByKeyword(context.Background(), "存在しない", 10)
	require.NoError(t, err)
	assert.Empty(t, nodes)
	assert.Empty(t, logs)
}

func TestGBizClient_SearchByKeyword_FiltersMissingCorporateNumber(t *testing.T) {
	// corporate_number が空のレコードはフィルタアウトされること
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse([]map[string]any{
			{"corporate_number": "1111111111111", "name": "正常な企業"},
			{"corporate_number": "", "name": "番号なし企業"}, // フィルタ対象
			{"name": "番号フィールドなし企業"},                    // フィルタ対象
		})))
	})

	nodes, _, err := client.SearchByKeyword(context.Background(), "test", 10)
	require.NoError(t, err)
	require.Len(t, nodes, 1, "corporate_number がない企業はフィルタされること")
	assert.Equal(t, "正常な企業", nodes[0].OfficialName)
}

func TestGBizClient_SearchByKeyword_DefaultLimit(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		// limit <= 0 の場合、デフォルト値 20 が使われること
		assert.Equal(t, "20", r.URL.Query().Get("limit"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse(nil)))
	})

	_, _, err := client.SearchByKeyword(context.Background(), "test", 0)
	require.NoError(t, err)
}

func TestGBizClient_SearchByKeyword_Unauthorized(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	nodes, _, err := client.SearchByKeyword(context.Background(), "test", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401 Unauthorized")
	assert.Nil(t, nodes)
}

func TestGBizClient_SearchByKeyword_LogsContainCompanyInfo(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse([]map[string]any{
			{"corporate_number": "1234567890123", "name": "ログ確認株式会社"},
		})))
	})

	_, logs, err := client.SearchByKeyword(context.Background(), "test", 10)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Contains(t, logs[0], "ログ確認株式会社")
	assert.Contains(t, logs[0], "1234567890123")
}
