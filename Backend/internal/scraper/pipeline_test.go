package scraper_test

import (
	"context"
	"net/http"
	"testing"

	"Backend/internal/scraper"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Pipeline.Run() ────────────────────────────────────────────────────────────

func TestPipeline_Run_Success(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "トヨタ", r.URL.Query().Get("name"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse([]map[string]any{
			{
				"corporate_number": "4800000000001",
				"name":             "トヨタ自動車株式会社",
				"location":         "愛知県豊田市",
				"business_summary": map[string]any{"major_classification_name": "製造業"},
				"company_url":      "https://www.toyota.co.jp",
			},
			{
				"corporate_number": "4800000000002",
				"name":             "トヨタファイナンシャルサービス株式会社",
				"location":         "愛知県名古屋市",
				"business_summary": map[string]any{"major_classification_name": "金融業"},
				"company_url":      "https://www.toyota-fs.co.jp",
			},
		})))
	})

	pipeline := &scraper.Pipeline{GBiz: client}
	result, err := pipeline.Run(context.Background(), scraper.RunRequest{
		Query:    "トヨタ",
		MaxPages: 1,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Nodes, 2)
	assert.NotEmpty(t, result.Logs)

	node := result.Nodes["4800000000001"]
	require.NotNil(t, node)
	assert.Equal(t, "トヨタ自動車株式会社", node.OfficialName)
	assert.Equal(t, "愛知県豊田市", node.Address)
	assert.Equal(t, "製造業", node.BusinessCategory)
	assert.Equal(t, 1.0, node.MatchScore)
	assert.False(t, node.NeedsReview)
}

func TestPipeline_Run_EmptyResults(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse([]map[string]any{})))
	})

	pipeline := &scraper.Pipeline{GBiz: client}
	result, err := pipeline.Run(context.Background(), scraper.RunRequest{
		Query:    "存在しないキーワード",
		MaxPages: 1,
	})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Nodes)
}

func TestPipeline_Run_NilGBizClient(t *testing.T) {
	// GBizClient が nil の場合はエラーを返すこと
	pipeline := &scraper.Pipeline{GBiz: nil}
	result, err := pipeline.Run(context.Background(), scraper.RunRequest{
		Query: "test",
	})

	require.Error(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Nodes)
}

func TestPipeline_Run_APIError(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	pipeline := &scraper.Pipeline{GBiz: client}
	result, err := pipeline.Run(context.Background(), scraper.RunRequest{
		Query:    "test",
		MaxPages: 1,
	})

	require.Error(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Nodes)
}

func TestPipeline_Run_SitesFieldIgnored(t *testing.T) {
	// Sites フィールドは内部パイプラインでは無視されること（後方互換維持）
	callCount := 0
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse([]map[string]any{
			{"corporate_number": "1111111111111", "name": "テスト株式会社"},
		})))
	})

	pipeline := &scraper.Pipeline{GBiz: client}
	result, err := pipeline.Run(context.Background(), scraper.RunRequest{
		Sites:    []string{"mynavi", "rikunabi", "career_tasu"}, // 無視される
		Query:    "テスト",
		MaxPages: 1,
	})

	require.NoError(t, err)
	assert.Len(t, result.Nodes, 1, "Sites フィールドに関係なく gBizINFO 検索結果が返ること")
	assert.Equal(t, 1, callCount, "gBizINFO API が1回だけ呼ばれること")
}

func TestPipeline_Run_YearOverride(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse(nil)))
	})

	pipeline := &scraper.Pipeline{GBiz: client}
	result, err := pipeline.Run(context.Background(), scraper.RunRequest{
		Query: "test",
		Year:  2030, // 明示的な年度指定
	})

	require.NoError(t, err)
	assert.Equal(t, 2030, result.TargetYear)
}

func TestPipeline_Run_MaxPagesAffectsLimit(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		// MaxPages=3 → limit=30 となること
		assert.Equal(t, "30", r.URL.Query().Get("limit"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse(nil)))
	})

	pipeline := &scraper.Pipeline{GBiz: client}
	_, err := pipeline.Run(context.Background(), scraper.RunRequest{
		Query:    "test",
		MaxPages: 3,
	})
	require.NoError(t, err)
}

func TestPipeline_Run_ContextCancellation(t *testing.T) {
	_, client := newGBizServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(gbizResponse(nil)))
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 即座にキャンセル

	pipeline := &scraper.Pipeline{GBiz: client}
	_, err := pipeline.Run(ctx, scraper.RunRequest{Query: "test"})
	// キャンセル済みコンテキストではエラーが返るべき
	require.Error(t, err)
}
