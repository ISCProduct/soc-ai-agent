package company_test

import (
	"context"
	"errors"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/company"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestProvisionByName_CreatesCompany は取得結果がDBに保存されることを検証する（#1124）。
//
// 従来は Web検索で実在確認だけを行い結果を捨てていたため、次に同じ企業名が来れば
// また払っていた。保存されることがこの変更の主目的。
func TestProvisionByName_CreatesCompany(t *testing.T) {
	srv := makeChatCompletionsServer(t, validCompanyInfoJSON())
	defer srv.Close()

	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByName", "テスト株式会社").Return(nil, errors.New("not found"))
	repo.On("Create", mock.AnythingOfType("*models.Company")).Return(nil).Run(func(args mock.Arguments) {
		c := args.Get(0).(*models.Company)
		assert.Equal(t, "テスト株式会社", c.Name)
		assert.Equal(t, "テスト企業の概要", c.Description)
		// 出どころが記録されること（#1303 の出どころバッジがこれを読む）
		assert.Equal(t, "web_search", c.SourceType)
		assert.Equal(t, "medium", c.LastFetchConfidence)
		// 取得直後は暫定
		assert.True(t, c.IsProvisional)
		assert.Equal(t, "draft", c.DataStatus)
		assert.NotNil(t, c.InfoFetchedAt)
	})

	client := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	fetcher := company.NewCompanyInfoFetcher(repo, client)

	got, err := fetcher.ProvisionByName(context.Background(), "テスト株式会社")
	require.NoError(t, err)
	assert.Equal(t, "テスト株式会社", got.Name)
	repo.AssertCalled(t, "Create", mock.AnythingOfType("*models.Company"))
}

// 既にDBにあれば取得しない（二重登録と無駄な課金を防ぐ）
func TestProvisionByName_ExistingCompanyIsNotFetched(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByName", "既存株式会社").Return(&models.Company{ID: 7, Name: "既存株式会社"}, nil)

	// client を nil にしておく。取得へ進んだらパニックか失敗になるので検出できる
	fetcher := company.NewCompanyInfoFetcher(repo, nil)

	got, err := fetcher.ProvisionByName(context.Background(), "既存株式会社")
	require.NoError(t, err)
	assert.Equal(t, uint(7), got.ID)
	repo.AssertNotCalled(t, "Create", mock.Anything)
}

// TestProvisionByName_RejectsThinResult は中身の薄い結果で企業を作らないことを検証する。
//
// 事業内容が無い企業を登録すると、打ち間違えた名前のレコードが学生の企業検索に載り、
// かつ企業briefの中核が空なのでレビューは一般論のままになる。
func TestProvisionByName_RejectsThinResult(t *testing.T) {
	// 業種と公式URLはあるが事業内容が無い
	thin := `{"industry":"IT・ソフトウェア","website_url":"https://example.com"}`
	srv := makeChatCompletionsServer(t, thin)
	defer srv.Close()

	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByName", mock.Anything).Return(nil, errors.New("not found"))

	client := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	fetcher := company.NewCompanyInfoFetcher(repo, client)

	_, err := fetcher.ProvisionByName(context.Background(), "架空株式会社XYZ999")
	require.Error(t, err)
	repo.AssertNotCalled(t, "Create", mock.Anything)
}

// TestProvisionByName_NegativeCache は失敗した企業名を再試行しないことを検証する（#1124）。
//
// これが無いと、打ち間違えた企業名を投げるたびに gBizinfo + Web検索がフルで走る。
// singleflight は同時実行しかまとめないので、間隔を空けた再送は素通りする。
func TestProvisionByName_NegativeCache(t *testing.T) {
	var calls int
	srv := makeCountingServer(t, `{"industry":"IT"}`, &calls)
	defer srv.Close()

	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByName", mock.Anything).Return(nil, errors.New("not found"))

	client := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	fetcher := company.NewCompanyInfoFetcher(repo, client)

	_, err1 := fetcher.ProvisionByName(context.Background(), "架空株式会社XYZ999")
	require.Error(t, err1)
	afterFirst := calls
	require.Greater(t, afterFirst, 0, "1回目は取得を試みるはず")

	_, err2 := fetcher.ProvisionByName(context.Background(), "架空株式会社XYZ999")
	require.Error(t, err2)
	assert.Equal(t, afterFirst, calls, "2回目は取得を試みてはいけない（ネガティブキャッシュ）")
}

// 表記ゆれも同じ失敗として扱う（normalizeCompanyKey で正規化される）
func TestProvisionByName_NegativeCacheNormalizesName(t *testing.T) {
	var calls int
	srv := makeCountingServer(t, `{"industry":"IT"}`, &calls)
	defer srv.Close()

	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByName", mock.Anything).Return(nil, errors.New("not found"))

	client := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	fetcher := company.NewCompanyInfoFetcher(repo, client)

	_, _ = fetcher.ProvisionByName(context.Background(), "架空XYZ999")
	afterFirst := calls

	_, _ = fetcher.ProvisionByName(context.Background(), "株式会社架空XYZ999")
	assert.Equal(t, afterFirst, calls, "株式会社の有無だけの違いで再取得してはいけない")
}
