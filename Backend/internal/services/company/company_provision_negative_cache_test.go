package company

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/openai"
)

// provisionRepoStub は FindByName が常に「未登録」を返す最小スタブ。
// ProvisionByName の取得経路へ必ず落とすために使う。
type provisionRepoStub struct {
	warmRepoStub
	findByNameCalls int
	created         *models.Company
}

func (s *provisionRepoStub) Create(c *models.Company) error {
	s.created = c
	return nil
}

func (s *provisionRepoStub) FindByName(string) (*models.Company, error) {
	s.findByNameCalls++
	return nil, nil
}

// TestProvisionByName_DoesNotCacheFetchErrors は、取得エラーをネガティブキャッシュに
// 記録しないことを検証する（#1124 のレビュー指摘）。
//
// タイムアウト・ユーザー離脱・プロバイダの 5xx は「その企業が存在しない」証拠ではない。
// 記録すると実在する企業が1時間ブロックされ、しかも DB に無いので企業検索からも
// 選べず行き止まりになる。
func TestProvisionByName_DoesNotCacheFetchErrors(t *testing.T) {
	repo := &provisionRepoStub{}
	// llm.Client が nil なので acquireViaAISearch はエラーを返す。
	// 「こちら側の都合で取得できなかった」ケースの代表。
	f := NewCompanyInfoFetcher(repo, nil)

	if _, err := f.ProvisionByName(context.Background(), "株式会社テスト"); err == nil {
		t.Fatal("error = nil, want error")
	}
	if f.recentlyFailedProvision(normalizeCompanyKey("株式会社テスト")) {
		t.Fatal("取得エラーを記録している（実在企業が1時間ブロックされる）")
	}

	// 2回目も取得まで到達すること。記録されていれば手前で弾かれて FindByName が増えない。
	before := repo.findByNameCalls
	if _, err := f.ProvisionByName(context.Background(), "株式会社テスト"); err == nil {
		t.Fatal("error = nil, want error")
	}
	if repo.findByNameCalls == before {
		t.Error("2回目が取得へ到達していない（エラーがキャッシュされている）")
	}
}

// キャンセル済みコンテキストでも同じ。ユーザーが画面を閉じただけ。
func TestProvisionByName_DoesNotCacheCancellation(t *testing.T) {
	f := NewCompanyInfoFetcher(&provisionRepoStub{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := f.ProvisionByName(ctx, "株式会社テスト"); err == nil {
		t.Fatal("error = nil, want error")
	}
	if f.recentlyFailedProvision(normalizeCompanyKey("株式会社テスト")) {
		t.Error("キャンセルを記録している")
	}
}

// 直近で失敗した名前は取得へ落とさない。打ち間違いを繰り返し投げられたときに
// 毎回 Web検索を払わないためのネガティブキャッシュ。
func TestProvisionByName_SkipsRecentlyFailedName(t *testing.T) {
	repo := &provisionRepoStub{}
	f := NewCompanyInfoFetcher(repo, nil)
	f.markProvisionFailed(normalizeCompanyKey("株式会社テスト"))

	_, err := f.ProvisionByName(context.Background(), "株式会社テスト")
	if err == nil {
		t.Fatal("error = nil, want error")
	}
	if !strings.Contains(err.Error(), "直近") {
		t.Errorf("ネガティブキャッシュ由来のエラーになっていない: %v", err)
	}
	if repo.findByNameCalls != 0 {
		t.Errorf("記録済みの名前で取得へ落ちている: FindByName %d回", repo.findByNameCalls)
	}
}

// 記録は正規化後のキーで引く。表記ゆれで素通りすると意味がない。
func TestProvisionFailure_UsesNormalizedKey(t *testing.T) {
	repo := &provisionRepoStub{}
	f := NewCompanyInfoFetcher(repo, nil)
	f.markProvisionFailed(normalizeCompanyKey("株式会社テスト"))

	if _, err := f.ProvisionByName(context.Background(), "  株式会社テスト  "); err == nil {
		t.Fatal("error = nil, want error")
	}
	if repo.findByNameCalls != 0 {
		t.Errorf("前後空白の違いで素通りしている: FindByName %d回", repo.findByNameCalls)
	}
}

// TTL を過ぎたら再試行する。恒久的にブロックすると、
// 取得側の一時障害で落ちた企業が永久に登録できなくなる。
func TestProvisionFailure_ExpiresAfterTTL(t *testing.T) {
	repo := &provisionRepoStub{}
	f := NewCompanyInfoFetcher(repo, nil)
	key := normalizeCompanyKey("株式会社テスト")
	f.markProvisionFailed(key)

	f.provisionMu.Lock()
	f.provisionFailures[key] = time.Now().Add(-provisionFailureTTL - time.Second)
	f.provisionMu.Unlock()

	if f.recentlyFailedProvision(key) {
		t.Fatal("TTL を過ぎても再試行しない")
	}
	if _, err := f.ProvisionByName(context.Background(), "株式会社テスト"); err == nil {
		t.Fatal("error = nil, want error")
	}
	if repo.findByNameCalls == 0 {
		t.Error("TTL 経過後に取得へ落ちていない")
	}
}

// 空の企業名は取得へ落とさない。
func TestProvisionByName_EmptyName(t *testing.T) {
	repo := &provisionRepoStub{}
	f := NewCompanyInfoFetcher(repo, nil)

	if _, err := f.ProvisionByName(context.Background(), "   "); err == nil {
		t.Fatal("error = nil, want error")
	}
	if repo.findByNameCalls != 0 {
		t.Errorf("空名で取得へ落ちている: FindByName %d回", repo.findByNameCalls)
	}
}

// emptySearchProvider は検索は成功するが中身の無い結果を返す。
// 打ち間違いなど「実在しない企業名」を投げたときの挙動を再現する。
type emptySearchProvider struct{ calls int }

func (p *emptySearchProvider) Name() string { return "stub" }
func (p *emptySearchProvider) Search(context.Context, string, int) (string, string, error) {
	p.calls++
	return "該当する企業の情報は見つかりませんでした。", "stub-search", nil
}

// emptyParseServer は Parse 段で「全項目が空」の企業情報を返すサーバ。
func emptyParseServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role": "assistant",
					// description も main_business も空 = companyInfoIsSubstantive が false
					"content": `{"description":"","main_business":"","industry":"","location":""}`,
				},
				"finish_reason": "stop",
			}},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestProvisionByName_CachesEmptyResult は、取得は成功したのに中身が無い場合だけ
// ネガティブキャッシュを効かせることを検証する（#1124 のレビュー指摘）。
//
// これが「実在しない企業名」の唯一の signal。ここを記録しないと、打ち間違いを
// 繰り返し投げられるたびに Web検索を払い続けることになる。
func TestProvisionByName_CachesEmptyResult(t *testing.T) {
	repo := &provisionRepoStub{}
	srv := emptyParseServer(t)

	f := NewCompanyInfoFetcher(repo, openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"))
	search := &emptySearchProvider{}
	f.llm.Search = search

	if _, err := f.ProvisionByName(context.Background(), "存在しない企業名"); err == nil {
		t.Fatal("error = nil, want error")
	}
	if search.calls != 1 {
		t.Fatalf("検索の呼び出し回数 = %d, want 1", search.calls)
	}
	if !f.recentlyFailedProvision(normalizeCompanyKey("存在しない企業名")) {
		t.Fatal("中身の無い結果を記録していない（打ち間違いのたびに検索を払う）")
	}

	// 2回目は手前で弾かれ、検索まで到達しない。これがコスト削減の実体。
	if _, err := f.ProvisionByName(context.Background(), "存在しない企業名"); err == nil {
		t.Fatal("error = nil, want error")
	}
	if search.calls != 1 {
		t.Errorf("2回目も検索している: 呼び出し回数 = %d, want 1", search.calls)
	}
}

// 中身のある結果は記録せず、企業として登録する。
func TestProvisionByName_SubstantiveResultIsNotCached(t *testing.T) {
	repo := &provisionRepoStub{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"role":    "assistant",
					"content": `{"description":"BtoB SaaSの開発","main_business":"BtoB SaaSの開発","industry":"情報通信業"}`,
				},
				"finish_reason": "stop",
			}},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	f := NewCompanyInfoFetcher(repo, openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"))
	f.llm.Search = &emptySearchProvider{}

	if _, err := f.ProvisionByName(context.Background(), "株式会社テスト"); err != nil {
		t.Fatalf("error = %v", err)
	}
	if f.recentlyFailedProvision(normalizeCompanyKey("株式会社テスト")) {
		t.Error("成功した企業名を記録している")
	}
	if repo.created == nil {
		t.Fatal("企業が登録されていない")
	}
	if !repo.created.IsProvisional || repo.created.DataStatus != "draft" {
		t.Errorf("取得直後は暫定・下書きであること: provisional=%v status=%q",
			repo.created.IsProvisional, repo.created.DataStatus)
	}
}

// TTL が実運用に耐える長さであること。
// 長すぎると一時障害で落ちた企業が実質永久にブロックされ、
// 短すぎるとネガティブキャッシュの意味が無い。
func TestProvisionFailureTTL_IsSane(t *testing.T) {
	if provisionFailureTTL < time.Minute {
		t.Errorf("provisionFailureTTL = %v: 短すぎて再送を止められない", provisionFailureTTL)
	}
	if provisionFailureTTL > 24*time.Hour {
		t.Errorf("provisionFailureTTL = %v: 長すぎて一時障害の企業がブロックされ続ける", provisionFailureTTL)
	}
}
