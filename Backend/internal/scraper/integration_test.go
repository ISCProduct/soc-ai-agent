package scraper_test

// 統合テスト: 実際の gBizINFO API を使用して動作確認を行う。
//
// 実行方法:
//   GBIZINFO_API_TOKEN=<your-token> go test ./internal/scraper/... -run Integration -v
//
// トークン未設定の場合は全テストが自動的にスキップされる。
// gBizINFO APIトークンの取得: https://info.gbiz.go.jp/

import (
	"context"
	"os"
	"testing"
	"time"

	"Backend/internal/scraper"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoToken は GBIZINFO_API_TOKEN が未設定の場合テストをスキップする。
func skipIfNoToken(t *testing.T) string {
	t.Helper()
	token := os.Getenv("GBIZINFO_API_TOKEN")
	if token == "" {
		t.Skip("GBIZINFO_API_TOKEN が未設定のためスキップ（統合テストには実トークンが必要）")
	}
	return token
}

// newRealClient は実 gBizINFO API クライアントを返す。
func newRealClient(t *testing.T) *scraper.GBizClient {
	t.Helper()
	token := skipIfNoToken(t)
	return scraper.NewGBizClient("", token)
}

// ── GBizClient 統合テスト ────────────────────────────────────────────────────

func TestIntegration_GBizClient_Search_Toyota(t *testing.T) {
	client := newRealClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	records, err := client.Search(ctx, "トヨタ自動車", "")

	require.NoError(t, err, "gBizINFO APIへの接続・認証が成功すること")
	require.NotEmpty(t, records, "トヨタ自動車の検索結果が1件以上返ること")

	t.Logf("検索結果: %d 件", len(records))
	for i, r := range records {
		t.Logf("  [%d] %s (%s) - %s", i+1, r.Name, r.CorporateNumber, r.Location)
	}

	// 法人番号・名称の存在確認
	assert.NotEmpty(t, records[0].CorporateNumber, "法人番号が返ること")
	assert.NotEmpty(t, records[0].Name, "企業名が返ること")
}

func TestIntegration_GBizClient_SearchByKeyword_IT(t *testing.T) {
	client := newRealClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	nodes, logs, err := client.SearchByKeyword(ctx, "システム開発", 5)

	require.NoError(t, err, "SearchByKeyword が正常終了すること")
	t.Logf("取得件数: %d 件", len(nodes))
	t.Logf("ログ:\n  %v", logs)

	for _, node := range nodes {
		// 全ノードの基本フィールドを検証
		assert.NotEmpty(t, node.CorporateNumber, "法人番号が設定されていること")
		assert.NotEmpty(t, node.OfficialName, "企業名が設定されていること")
		assert.Equal(t, 1.0, node.MatchScore, "gBizINFO直接取得のためMatchScore=1.0であること")
		assert.False(t, node.NeedsReview, "gBizINFO直接取得のためNeedsReview=falseであること")
		t.Logf("  - %s [%s] %s", node.OfficialName, node.CorporateNumber, node.BusinessCategory)
	}
}

func TestIntegration_GBizClient_Search_CorporateNumberFormat(t *testing.T) {
	client := newRealClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	records, err := client.Search(ctx, "株式会社", "")
	require.NoError(t, err)

	for _, r := range records {
		if r.CorporateNumber != "" {
			// 法人番号は13桁であること
			assert.Len(t, r.CorporateNumber, 13,
				"法人番号 %q は13桁であること", r.CorporateNumber)
		}
	}
}

// ── Pipeline 統合テスト ──────────────────────────────────────────────────────

func TestIntegration_Pipeline_Run_WithRealAPI(t *testing.T) {
	client := newRealClient(t)
	pipeline := &scraper.Pipeline{GBiz: client}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := pipeline.Run(ctx, scraper.RunRequest{
		Query:    "ソフトウェア",
		MaxPages: 1, // limit = 10件
	})

	require.NoError(t, err, "Pipeline.Run が正常終了すること")
	require.NotNil(t, result)

	t.Logf("対象年度: %d", result.TargetYear)
	t.Logf("取得企業数: %d 社", len(result.Nodes))
	t.Logf("ログ:\n  %v", result.Logs)

	// 年度の妥当性確認
	now := time.Now()
	assert.GreaterOrEqual(t, result.TargetYear, now.Year(),
		"対象年度は現在年以降であること")

	// 取得した企業の内容確認
	for corpNum, node := range result.Nodes {
		assert.Equal(t, corpNum, node.CorporateNumber,
			"マップのキーと CorporateNumber が一致すること")
		assert.NotEmpty(t, node.OfficialName, "企業名が存在すること")
		assert.Equal(t, 1.0, node.MatchScore)
		assert.False(t, node.NeedsReview)
		t.Logf("  [%s] %s / %s", node.CorporateNumber, node.OfficialName, node.BusinessCategory)
	}
}

func TestIntegration_Pipeline_Run_SitesFieldDoesNotAffectResult(t *testing.T) {
	client := newRealClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Sites 指定ありと Sites 指定なしで同じ結果になること
	pipeline := &scraper.Pipeline{GBiz: client}

	withSites, err := pipeline.Run(ctx, scraper.RunRequest{
		Query:    "製造",
		Sites:    []string{"mynavi", "rikunabi"}, // 内部では無視される
		MaxPages: 1,
	})
	require.NoError(t, err)

	// レート制限のため少し待機
	time.Sleep(1500 * time.Millisecond)

	withoutSites, err := pipeline.Run(ctx, scraper.RunRequest{
		Query:    "製造",
		MaxPages: 1,
	})
	require.NoError(t, err)

	assert.Equal(t, len(withSites.Nodes), len(withoutSites.Nodes),
		"Sites フィールドの有無が結果に影響しないこと")
	t.Logf("Sites指定あり: %d社 / Sites指定なし: %d社", len(withSites.Nodes), len(withoutSites.Nodes))
}
