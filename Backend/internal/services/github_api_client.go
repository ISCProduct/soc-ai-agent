package services

import (
	"Backend/internal/models"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// --- 内部ヘルパー ---

// githubAPIRepository GitHub API レスポンスのリポジトリ構造体
type githubAPIRepository struct {
	Name            string `json:"name"`
	FullName        string `json:"full_name"`
	Description     string `json:"description"`
	Language        string `json:"language"`
	StargazersCount int    `json:"stargazers_count"`
	ForksCount      int    `json:"forks_count"`
	Fork            bool   `json:"fork"`
	UpdatedAt       string `json:"updated_at"`
}

// fetchRepositories 自分のリポジトリ + 所属組織のリポジトリを取得してマージする
func (s *GitHubService) fetchRepositories(ctx context.Context, client *http.Client, token string) ([]models.GitHubRepo, error) {
	seen := make(map[string]struct{})

	// 1. /user/repos?affiliation=owner,collaborator,organization_member で全リポジトリを取得
	// affiliation を明示指定することで、オーナー・コラボレーター・組織メンバーとして参加している
	// リポジトリをすべて取得する（read:orgなしでも動く）
	allTypeRepos, err := s.FetchRepoPages(ctx, client, token,
		fmt.Sprintf("%s/user/repos?affiliation=owner,collaborator,organization_member&sort=updated&per_page=100", githubAPIBase))
	if err != nil {
		log.Printf("[GitHubService] fetchRepos(affiliation=all) warning: %v", err)
	}
	allRepos := allTypeRepos
	for _, r := range allTypeRepos {
		seen[r.FullName] = struct{}{}
	}

	// 2. 組織リポジトリも明示的に取得してマージ（repo + read:org スコープが必要）
	orgs, err := s.fetchOrgs(ctx, client, token)
	if err != nil {
		var scopeErr *InsufficientScopesError
		if errors.As(err, &scopeErr) {
			// スコープ不足は呼び出し元に伝播してユーザーに再認証を促す
			return nil, scopeErr
		}
		log.Printf("[GitHubService] fetchOrgs warning: %v", err)
	} else {
		for _, org := range orgs {
			orgRepos, err := s.FetchRepoPages(ctx, client, token,
				fmt.Sprintf("%s/orgs/%s/repos?type=all&sort=updated&per_page=100", githubAPIBase, org))
			if err != nil {
				log.Printf("[GitHubService] fetchOrgRepos warning (%s): %v", org, err)
				continue
			}
			for _, r := range orgRepos {
				if _, exists := seen[r.FullName]; !exists {
					seen[r.FullName] = struct{}{}
					allRepos = append(allRepos, r)
				}
			}
		}
	}

	// 3. GraphQL repositoriesContributedTo でコントリビュート済みリポジトリを追加取得
	// REST APIの組織承認制限を回避し、参加しているすべての組織リポジトリを取得できる
	contributedRepos, err := s.fetchContributedRepos(ctx, client, token)
	if err != nil {
		log.Printf("[GitHubService] fetchContributedRepos warning: %v", err)
	} else {
		for _, r := range contributedRepos {
			if _, exists := seen[r.FullName]; !exists {
				seen[r.FullName] = struct{}{}
				allRepos = append(allRepos, r)
			}
		}
	}

	return allRepos, nil
}

// InsufficientScopesError トークンのスコープ不足エラー型
type InsufficientScopesError struct {
	Missing []string
}

func (e *InsufficientScopesError) Error() string {
	return fmt.Sprintf("GitHubトークンに必要なスコープが不足しています（%s）。GitHubアカウントを再連携してください。",
		strings.Join(e.Missing, ", "))
}

// hasScopes トークンが必要なスコープをすべて持っているか確認する
func hasScopes(scopeHeader string, required ...string) []string {
	var missing []string
	for _, r := range required {
		if !strings.Contains(scopeHeader, r) {
			missing = append(missing, r)
		}
	}
	return missing
}

// fetchOrgs 認証ユーザーの所属組織名一覧を取得する
func (s *GitHubService) fetchOrgs(ctx context.Context, client *http.Client, token string) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/user/orgs?per_page=100", githubAPIBase), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	scopes := resp.Header.Get("X-OAuth-Scopes")
	log.Printf("[GitHubService] token scopes: %q", scopes)

	// repo と read:org の両方が必要
	if missing := hasScopes(scopes, "repo", "read:org"); len(missing) > 0 {
		return nil, &InsufficientScopesError{Missing: missing}
	}

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var orgs []struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &orgs); err != nil {
		return nil, err
	}
	names := make([]string, len(orgs))
	for i, o := range orgs {
		names[i] = o.Login
	}
	return names, nil
}

// FetchRepoPages ページネーションで全リポジトリを取得する
func (s *GitHubService) FetchRepoPages(ctx context.Context, client *http.Client, token, baseURL string) ([]models.GitHubRepo, error) {
	var allRepos []models.GitHubRepo
	page := 1
	for {
		url := fmt.Sprintf("%s&page=%d", baseURL, page)
		body, err := s.doRequestWithRetry(ctx, client, token, url)
		if err != nil {
			return nil, err
		}
		var apiRepos []githubAPIRepository
		if err := json.Unmarshal(body, &apiRepos); err != nil {
			return nil, fmt.Errorf("unmarshal repos page %d: %w", page, err)
		}
		if len(apiRepos) == 0 {
			break
		}
		for _, r := range apiRepos {
			updatedAt, _ := time.Parse(time.RFC3339, r.UpdatedAt)
			allRepos = append(allRepos, models.GitHubRepo{
				Name:            r.Name,
				FullName:        r.FullName,
				Description:     r.Description,
				Language:        r.Language,
				Stars:           r.StargazersCount,
				Forks:           r.ForksCount,
				IsForked:        r.Fork,
				GitHubUpdatedAt: updatedAt,
			})
		}
		if len(apiRepos) < 100 {
			break
		}
		page++
	}
	return allRepos, nil
}

// githubLanguagesResponse GitHub言語APIレスポンス（言語名→バイト数のmap）
type githubLanguagesResponse map[string]int64

// AggregateLanguages リポジトリ一覧から言語使用統計を集計する
func AggregateLanguages(userID uint, repos []models.GitHubRepo) []models.GitHubLanguageStat {
	langBytes := make(map[string]int64)
	var total int64

	for _, r := range repos {
		if r.Language != "" {
			// リポジトリのメイン言語のみ集計（バイト数は不明なので件数ベース）
			langBytes[r.Language]++
			total++
		}
	}

	if total == 0 {
		return nil
	}

	stats := make([]models.GitHubLanguageStat, 0, len(langBytes))
	for lang, count := range langBytes {
		stats = append(stats, models.GitHubLanguageStat{
			UserID:     userID,
			Language:   lang,
			Bytes:      count,
			Percentage: float64(count) / float64(total) * 100,
		})
	}
	return stats
}

// contributedReposGraphQLResponse GraphQL repositoriesContributedTo レスポンス
type contributedReposGraphQLResponse struct {
	Data struct {
		Viewer struct {
			RepositoriesContributedTo struct {
				Nodes []struct {
					Name            string `json:"name"`
					NameWithOwner   string `json:"nameWithOwner"`
					Description     string `json:"description"`
					PrimaryLanguage *struct {
						Name string `json:"name"`
					} `json:"primaryLanguage"`
					StargazerCount int    `json:"stargazerCount"`
					ForkCount      int    `json:"forkCount"`
					IsFork         bool   `json:"isFork"`
					UpdatedAt      string `json:"updatedAt"`
				} `json:"nodes"`
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"repositoriesContributedTo"`
		} `json:"viewer"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// fetchContributedRepos GraphQL APIで自分がコントリビュートしたリポジトリ一覧を取得する
// REST APIと異なり組織のOAuthアプリ承認不要でプライベート組織リポジトリも取得できる
func (s *GitHubService) fetchContributedRepos(ctx context.Context, client *http.Client, token string) ([]models.GitHubRepo, error) {
	var allRepos []models.GitHubRepo
	after := ""

	for {
		// GraphQL variables を使いカーソル値を安全に渡す（文字列インジェクション防止）
		type contributedReposVars struct {
			After *string `json:"after,omitempty"`
		}
		vars := contributedReposVars{}
		if after != "" {
			vars.After = &after
		}
		gqlPayload := struct {
			Query     string               `json:"query"`
			Variables contributedReposVars `json:"variables"`
		}{
			Query:     `query($after: String) { viewer { repositoriesContributedTo(first: 100, includeUserRepositories: true, contributionTypes: [COMMIT, PULL_REQUEST, REPOSITORY], after: $after) { nodes { name nameWithOwner description primaryLanguage { name } stargazerCount forkCount isFork updatedAt } pageInfo { hasNextPage endCursor } } } }`,
			Variables: vars,
		}
		queryBytes, err := json.Marshal(gqlPayload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal graphql query: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubGraphQLURL, bytes.NewReader(queryBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		var result contributedReposGraphQLResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("unmarshal contributedRepos: %w", err)
		}
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("graphql error: %s", result.Errors[0].Message)
		}

		for _, n := range result.Data.Viewer.RepositoriesContributedTo.Nodes {
			lang := ""
			if n.PrimaryLanguage != nil {
				lang = n.PrimaryLanguage.Name
			}
			updatedAt, _ := time.Parse(time.RFC3339, n.UpdatedAt)
			allRepos = append(allRepos, models.GitHubRepo{
				Name:            n.Name,
				FullName:        n.NameWithOwner,
				Description:     n.Description,
				Language:        lang,
				Stars:           n.StargazerCount,
				Forks:           n.ForkCount,
				IsForked:        n.IsFork,
				GitHubUpdatedAt: updatedAt,
			})
		}

		if !result.Data.Viewer.RepositoriesContributedTo.PageInfo.HasNextPage {
			break
		}
		after = result.Data.Viewer.RepositoriesContributedTo.PageInfo.EndCursor
	}

	return allRepos, nil
}

// contributionsGraphQLResponse GraphQLレスポンス構造体
type contributionsGraphQLResponse struct {
	Data struct {
		User struct {
			ContributionsCollection struct {
				ContributionCalendar struct {
					TotalContributions int `json:"totalContributions"`
				} `json:"contributionCalendar"`
			} `json:"contributionsCollection"`
		} `json:"user"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// FetchTotalContributions GraphQL APIで年間コントリビューション数を取得する
func (s *GitHubService) FetchTotalContributions(ctx context.Context, client *http.Client, token, login string) (int, error) {
	// GraphQL variables を使い login 値を安全に渡す（文字列インジェクション防止）
	gqlPayload := struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}{
		Query:     `query($login: String!) { user(login: $login) { contributionsCollection { contributionCalendar { totalContributions } } } }`,
		Variables: map[string]any{"login": login},
	}
	queryBytes, err := json.Marshal(gqlPayload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal graphql query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.getGraphQLURL(), bytes.NewReader(queryBytes))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if err := CheckRateLimit(resp); err != nil {
		return 0, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result contributionsGraphQLResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("unmarshal graphql response: %w", err)
	}
	if len(result.Errors) > 0 {
		return 0, fmt.Errorf("graphql error: %s", result.Errors[0].Message)
	}

	return result.Data.User.ContributionsCollection.ContributionCalendar.TotalContributions, nil
}

// doRequestWithRetry レート制限対応GETリクエスト（最大maxRetriesリトライ）
func (s *GitHubService) doRequestWithRetry(ctx context.Context, client *http.Client, token, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		body, err := s.doGet(ctx, client, token, url)
		if err == nil {
			return body, nil
		}
		lastErr = err

		// レート制限エラーの場合はリトライしない
		if IsRateLimitError(err) {
			return nil, err
		}

		if attempt < maxRetries {
			wait := time.Duration(attempt+1) * 2 * time.Second
			log.Printf("[GitHubService] request failed (attempt %d/%d), retrying in %s: %v", attempt+1, maxRetries, wait, err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	return nil, lastErr
}

// doGet GitHub APIへGETリクエストを送る
func (s *GitHubService) doGet(ctx context.Context, client *http.Client, token, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := CheckRateLimit(resp); err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api error: status %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	return io.ReadAll(resp.Body)
}

// rateLimitError レート制限エラー型
type rateLimitError struct {
	resetAt time.Time
}

func (e *rateLimitError) Error() string {
	return fmt.Sprintf("github rate limit exceeded, resets at %s", e.resetAt.Format(time.RFC3339))
}

func IsRateLimitError(err error) bool {
	_, ok := err.(*rateLimitError)
	return ok
}

// CheckRateLimit レスポンスヘッダーからレート制限を確認する
func CheckRateLimit(resp *http.Response) error {
	if resp.StatusCode != 403 && resp.StatusCode != 429 {
		return nil
	}
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	if remaining == "0" || resp.StatusCode == 429 {
		resetUnix, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
		resetAt := time.Unix(resetUnix, 0)
		return &rateLimitError{resetAt: resetAt}
	}
	return nil
}
