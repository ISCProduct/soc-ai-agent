package github_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/services/github"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// githubAPIRepositoryTest は fetchRepoPages モックサーバー用のローカル型です。
// (本体の githubAPIRepository は unexported のため同一フィールドで定義)
type githubAPIRepositoryTest struct {
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	Language        string `json:"language"`
	StargazersCount int    `json:"stargazers_count"`
	UpdatedAt       string `json:"updated_at"`
}

// --- aggregateLanguages ---

func TestAggregateLanguages_Empty(t *testing.T) {
	stats := github.AggregateLanguages(1, nil)
	assert.Nil(t, stats)
}

func TestAggregateLanguages_SkipsBlankLanguage(t *testing.T) {
	repos := []models.GitHubRepo{
		{Language: ""},
		{Language: ""},
	}
	stats := github.AggregateLanguages(1, repos)
	assert.Nil(t, stats, "repos with no language should produce no stats")
}

func TestAggregateLanguages_SingleLanguage(t *testing.T) {
	repos := []models.GitHubRepo{
		{Language: "Go"},
		{Language: "Go"},
		{Language: "Go"},
	}
	stats := github.AggregateLanguages(1, repos)
	require.Len(t, stats, 1)
	assert.Equal(t, "Go", stats[0].Language)
	assert.Equal(t, int64(3), stats[0].Bytes)
	assert.InDelta(t, 100.0, stats[0].Percentage, 0.01)
	assert.Equal(t, uint(1), stats[0].UserID)
}

func TestAggregateLanguages_MultipleLanguages(t *testing.T) {
	repos := []models.GitHubRepo{
		{Language: "Go"},
		{Language: "Go"},
		{Language: "TypeScript"},
		{Language: "Python"},
	}
	stats := github.AggregateLanguages(1, repos)
	require.Len(t, stats, 3)

	statMap := make(map[string]models.GitHubLanguageStat)
	for _, s := range stats {
		statMap[s.Language] = s
	}

	assert.Equal(t, int64(2), statMap["Go"].Bytes)
	assert.InDelta(t, 50.0, statMap["Go"].Percentage, 0.01)
	assert.InDelta(t, 25.0, statMap["TypeScript"].Percentage, 0.01)
	assert.InDelta(t, 25.0, statMap["Python"].Percentage, 0.01)
}

func TestAggregateLanguages_MixedWithBlank(t *testing.T) {
	repos := []models.GitHubRepo{
		{Language: "Go"},
		{Language: ""},
		{Language: "Python"},
	}
	stats := github.AggregateLanguages(1, repos)
	require.Len(t, stats, 2)
	statMap := make(map[string]models.GitHubLanguageStat)
	for _, s := range stats {
		statMap[s.Language] = s
	}
	assert.InDelta(t, 50.0, statMap["Go"].Percentage, 0.01)
	assert.InDelta(t, 50.0, statMap["Python"].Percentage, 0.01)
}

// --- checkRateLimit ---

func TestCheckRateLimit_NoRateLimit(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header)}
	assert.NoError(t, github.CheckRateLimit(resp))
}

func TestCheckRateLimit_404NotRateLimited(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header)}
	assert.NoError(t, github.CheckRateLimit(resp))
}

func TestCheckRateLimit_429(t *testing.T) {
	h := make(http.Header)
	h.Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()))
	resp := &http.Response{StatusCode: http.StatusTooManyRequests, Header: h}
	err := github.CheckRateLimit(resp)
	require.Error(t, err)
	assert.True(t, github.IsRateLimitError(err))
}

func TestCheckRateLimit_403WithRemainingZero(t *testing.T) {
	h := make(http.Header)
	h.Set("X-RateLimit-Remaining", "0")
	h.Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()))
	resp := &http.Response{StatusCode: http.StatusForbidden, Header: h}
	err := github.CheckRateLimit(resp)
	require.Error(t, err)
	assert.True(t, github.IsRateLimitError(err))
}

func TestCheckRateLimit_403WithRemainingNonZero(t *testing.T) {
	h := make(http.Header)
	h.Set("X-RateLimit-Remaining", "50")
	resp := &http.Response{StatusCode: http.StatusForbidden, Header: h}
	err := github.CheckRateLimit(resp)
	assert.NoError(t, err)
}

// --- fetchRepositories (httptest) ---

func TestFetchRepoPages_SinglePage(t *testing.T) {
	apiRepos := []githubAPIRepositoryTest{
		{Name: "repo1", FullName: "user/repo1", Language: "Go", StargazersCount: 5, UpdatedAt: "2024-01-01T00:00:00Z"},
		{Name: "repo2", FullName: "user/repo2", Language: "Python", StargazersCount: 2, UpdatedAt: "2024-01-02T00:00:00Z"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(apiRepos)
	}))
	defer server.Close()

	svc := github.NewGitHubServiceForTest("")
	client := &http.Client{Timeout: 5 * time.Second}

	repos, err := svc.FetchRepoPages(context.Background(), client, "token", server.URL+"/repos?per_page=100")
	require.NoError(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "repo1", repos[0].Name)
	assert.Equal(t, "user/repo1", repos[0].FullName)
	assert.Equal(t, "Go", repos[0].Language)
	assert.Equal(t, 5, repos[0].Stars)
}

func TestFetchRepoPages_MultiPage(t *testing.T) {
	page1 := make([]githubAPIRepositoryTest, 100)
	for i := range page1 {
		page1[i] = githubAPIRepositoryTest{Name: fmt.Sprintf("repo%d", i), FullName: fmt.Sprintf("user/repo%d", i), Language: "Go"}
	}
	page2 := make([]githubAPIRepositoryTest, 50)
	for i := range page2 {
		page2[i] = githubAPIRepositoryTest{Name: fmt.Sprintf("repo%d", i+100), FullName: fmt.Sprintf("user/repo%d", i+100), Language: "Python"}
	}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			json.NewEncoder(w).Encode(page1)
		} else {
			json.NewEncoder(w).Encode(page2)
		}
	}))
	defer server.Close()

	svc := github.NewGitHubServiceForTest("")
	client := &http.Client{Timeout: 5 * time.Second}

	repos, err := svc.FetchRepoPages(context.Background(), client, "token", server.URL+"/repos?per_page=100")
	require.NoError(t, err)
	assert.Len(t, repos, 150)
	assert.Equal(t, 2, callCount, "should have made 2 API calls for pagination")
}

func TestFetchRepoPages_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]githubAPIRepositoryTest{})
	}))
	defer server.Close()

	svc := github.NewGitHubServiceForTest("")
	client := &http.Client{Timeout: 5 * time.Second}

	repos, err := svc.FetchRepoPages(context.Background(), client, "token", server.URL+"/repos?per_page=100")
	require.NoError(t, err)
	assert.Empty(t, repos)
}

func TestFetchRepoPages_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	svc := github.NewGitHubServiceForTest("")
	client := &http.Client{Timeout: 5 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := svc.FetchRepoPages(ctx, client, "token", server.URL+"/repos?per_page=100")
	assert.Error(t, err)
}

// --- fetchTotalContributions (httptest) ---

func TestFetchTotalContributions_Success(t *testing.T) {
	respBody := `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"totalContributions":500}}}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, respBody)
	}))
	defer server.Close()

	svc := github.NewGitHubServiceForTest(server.URL)
	client := &http.Client{Timeout: 5 * time.Second}

	count, err := svc.FetchTotalContributions(context.Background(), client, "token", "user")
	require.NoError(t, err)
	assert.Equal(t, 500, count)
}

func TestFetchTotalContributions_GraphQLError(t *testing.T) {
	respBody := `{"errors":[{"message":"Could not resolve to a User with the login of 'unknown'."}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, respBody)
	}))
	defer server.Close()

	svc := github.NewGitHubServiceForTest(server.URL)
	client := &http.Client{Timeout: 5 * time.Second}

	_, err := svc.FetchTotalContributions(context.Background(), client, "token", "unknown")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "graphql error")
}
