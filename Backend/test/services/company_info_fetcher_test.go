package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services"
	"Backend/test/controllers/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func validCompanyInfoJSON() string {
	return `{"description":"テスト企業の概要","industry":"IT・ソフトウェア","location":"東京都渋谷区","website_url":"https://example.com","founded_year":2010,"employee_count":500,"main_business":"クラウドサービスの開発","culture":"フラットな組織文化","work_style":"ハイブリッド"}`
}

func makeChatCompletionsServer(t *testing.T, responseText string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "chat/completions") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": responseText}},
			},
			"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 20},
		})
	}))
}

func TestCompanyInfoFetcher_FetchAndSave_TTLCache(t *testing.T) {
	now := time.Now()
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Company{
		ID:            1,
		Name:          "テスト株式会社",
		Description:   "キャッシュ済み概要",
		Industry:      "IT",
		InfoFetchedAt: &now,
		SourceType:    "gbizinfo",
	}, nil)

	fetcher := services.NewCompanyInfoFetcher(repo, nil)
	result, err := fetcher.FetchAndSave(context.Background(), 1, false)
	require.NoError(t, err)
	assert.Equal(t, "キャッシュ済み概要", result.Description)
	repo.AssertNotCalled(t, "Update", mock.Anything)
}

func TestCompanyInfoFetcher_FetchAndSave_AISearchFallback(t *testing.T) {
	srv := makeChatCompletionsServer(t, validCompanyInfoJSON())
	defer srv.Close()

	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Company{
		ID:   1,
		Name: "テスト株式会社",
	}, nil)
	repo.On("Update", mock.AnythingOfType("*models.Company")).Return(nil).Run(func(args mock.Arguments) {
		c := args.Get(0).(*models.Company)
		assert.Equal(t, "web_search", c.SourceType)
		assert.NotNil(t, c.InfoFetchedAt)
		assert.Equal(t, "medium", c.LastFetchConfidence)
	})

	client := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	fetcher := services.NewCompanyInfoFetcher(repo, client) // gBiz なし → AI Search Lite
	result, err := fetcher.FetchAndSave(context.Background(), 1, true)
	require.NoError(t, err)
	assert.Equal(t, "テスト企業の概要", result.Description)
	assert.Equal(t, "web_search", result.Source)
}

func TestCompanyInfoFetcher_FetchAndSave_CompanyNotFound(t *testing.T) {
	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(99)).Return(nil, errors.New("record not found"))
	fetcher := services.NewCompanyInfoFetcher(repo, nil)
	_, err := fetcher.FetchAndSave(context.Background(), 99, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "company not found")
}

func TestCompanyInfoFetcher_FetchAndSave_UpdateError(t *testing.T) {
	srv := makeChatCompletionsServer(t, validCompanyInfoJSON())
	defer srv.Close()

	repo := &mocks.CompanyRepositoryMock{}
	repo.On("FindByID", uint(1)).Return(&models.Company{ID: 1, Name: "テスト株式会社"}, nil)
	repo.On("Update", mock.AnythingOfType("*models.Company")).Return(errors.New("db error"))

	client := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	fetcher := services.NewCompanyInfoFetcher(repo, client)
	_, err := fetcher.FetchAndSave(context.Background(), 1, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update company")
}

func TestCompanyInfoFetcher_Acquire_InvalidJSON(t *testing.T) {
	srv := makeChatCompletionsServer(t, "JSONではないテキスト")
	defer srv.Close()

	client := openai.NewWithBaseURL(srv.URL, "gpt-4o-mini")
	fetcher := services.NewCompanyInfoFetcher(nil, client)
	_, err := fetcher.Acquire(context.Background(), "テスト株式会社", "")
	require.Error(t, err)
}
