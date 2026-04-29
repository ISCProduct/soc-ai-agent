package services

import (
	"Backend/internal/crypto"
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/repositories"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	githubAPIBase    = "https://api.github.com"
	githubGraphQLURL = "https://api.github.com/graphql"
	// レート制限: 最終同期から1時間以内は再同期しない
	syncCacheDuration = time.Hour
	// レート制限リトライ: 最大2回
	maxRetries = 2
)

// GitHubService GitHub API連携サービス
type GitHubService struct {
	githubRepo        *repositories.GitHubRepository
	skillScoreService *SkillScoreService
	apiBaseURL        string // テスト用オーバーライド（空なら githubAPIBase を使用）
	graphQLURL        string // テスト用オーバーライド（空なら githubGraphQLURL を使用）
	openaiClient      *openai.Client
}

func NewGitHubService(githubRepo *repositories.GitHubRepository, skillScoreService *SkillScoreService, openaiClient *openai.Client) *GitHubService {
	return &GitHubService{
		githubRepo:        githubRepo,
		skillScoreService: skillScoreService,
		openaiClient:      openaiClient,
	}
}

// NewGitHubServiceForTest は graphQLURL を差し替えたテスト用 GitHubService を返します。
func NewGitHubServiceForTest(graphQLURL string) *GitHubService {
	return &GitHubService{graphQLURL: graphQLURL}
}

func (s *GitHubService) getAPIBase() string {
	if s.apiBaseURL != "" {
		return s.apiBaseURL
	}
	return githubAPIBase
}

func (s *GitHubService) getGraphQLURL() string {
	if s.graphQLURL != "" {
		return s.graphQLURL
	}
	return githubGraphQLURL
}

// StoreAccessToken GitHubアクセストークンを AES-GCM で暗号化してDBに保存する（#326）
func (s *GitHubService) StoreAccessToken(userID uint, login, accessToken string) error {
	encryptionKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
	storedToken := accessToken
	if encryptionKey != "" {
		encrypted, err := crypto.EncryptToken(accessToken, encryptionKey)
		if err != nil {
			log.Printf("[GitHubService] failed to encrypt access token for user %d: %v", userID, err)
			return fmt.Errorf("failed to encrypt access token: %w", err)
		}
		storedToken = encrypted
	} else {
		log.Printf("[GitHubService] WARNING: TOKEN_ENCRYPTION_KEY not set, storing access token in plaintext for user %d", userID)
	}
	profile := &models.GitHubProfile{
		UserID:      userID,
		GitHubLogin: login,
		AccessToken: storedToken,
	}
	return s.githubRepo.UpsertProfile(profile)
}

// TriggerAsyncSync 非同期でGitHubデータ同期を開始する（ノンブロッキング）
// force=true でキャッシュを無視して強制同期する
func (s *GitHubService) TriggerAsyncSync(userID uint, force bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.SyncUserData(ctx, userID, force); err != nil {
			log.Printf("[GitHubService] async sync failed for user %d: %v", userID, err)
		}
	}()
}

// SyncUserData GitHubからリポジトリ・言語比率・コントリビューション数を取得してDBに保存する
// force=true でキャッシュを無視して強制同期する
func (s *GitHubService) SyncUserData(ctx context.Context, userID uint, force bool) error {
	profile, err := s.githubRepo.GetProfile(userID)
	if err != nil {
		return fmt.Errorf("get profile: %w", err)
	}
	if profile == nil {
		return fmt.Errorf("github profile not found for user %d", userID)
	}

	// キャッシュチェック: 1時間以内に同期済みならスキップ（強制同期時はスキップしない）
	if !force && profile.SyncedAt != nil && time.Since(*profile.SyncedAt) < syncCacheDuration {
		log.Printf("[GitHubService] user %d: skipping sync (last synced %s ago)", userID, time.Since(*profile.SyncedAt).Round(time.Minute))
		return nil
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// トークンを復号する（暗号化されている場合）（#326）
	token := profile.AccessToken
	if encryptionKey := os.Getenv("TOKEN_ENCRYPTION_KEY"); encryptionKey != "" {
		decrypted, err := crypto.DecryptToken(profile.AccessToken, encryptionKey)
		if err != nil {
			log.Printf("[GitHubService] failed to decrypt access token for user %d: %v", userID, err)
			// 復号失敗時は平文として扱う（移行期対応）
		} else {
			token = decrypted
		}
	}
	login := profile.GitHubLogin

	// 1. リポジトリ一覧取得（自分のリポジトリ + 所属組織のリポジトリ）
	repos, err := s.fetchRepositories(ctx, client, token)
	if err != nil {
		return fmt.Errorf("fetch repositories: %w", err)
	}

	// 2. 言語使用比率集計
	langStats := AggregateLanguages(userID, repos)

	// userIDをセット
	for i := range repos {
		repos[i].UserID = userID
	}

	// 3. コントリビューション数取得（GraphQL）
	contributions, err := s.FetchTotalContributions(ctx, client, token, login)
	if err != nil {
		log.Printf("[GitHubService] fetch contributions warning: %v", err)
		// コントリビューション取得失敗はwarn扱いで続行
	}

	// 4. プロフィール統計更新
	now := time.Now()
	profile.TotalContributions = contributions
	profile.PublicRepos = len(repos)
	profile.SyncedAt = &now
	if err := s.githubRepo.UpsertProfile(profile); err != nil {
		return fmt.Errorf("update profile: %w", err)
	}

	// 5. リポジトリ保存
	if err := s.githubRepo.ReplaceRepositories(userID, repos); err != nil {
		return fmt.Errorf("save repositories: %w", err)
	}

	// 6. 言語比率保存
	if err := s.githubRepo.ReplaceLanguageStats(userID, langStats); err != nil {
		return fmt.Errorf("save language stats: %w", err)
	}

	// 7. スキルスコア算出・保存
	if s.skillScoreService != nil {
		if err := s.skillScoreService.CalculateAndSave(userID, langStats, repos, contributions); err != nil {
			log.Printf("[GitHubService] skill score calculation warning: %v", err)
		}
	}

	log.Printf("[GitHubService] user %d: sync completed (%d repos, %d contributions)", userID, len(repos), contributions)
	return nil
}

// GetProfile DBからGitHubプロフィールを取得する
func (s *GitHubService) GetProfile(userID uint) (*models.GitHubProfile, error) {
	return s.githubRepo.GetProfile(userID)
}

// GetRepositories DBからリポジトリ一覧を取得する
func (s *GitHubService) GetRepositories(userID uint) ([]models.GitHubRepo, error) {
	return s.githubRepo.GetRepositories(userID)
}

// GetLanguageStats DBから言語使用比率を取得する
func (s *GitHubService) GetLanguageStats(userID uint) ([]models.GitHubLanguageStat, error) {
	return s.githubRepo.GetLanguageStats(userID)
}

// ListRepoSummaries DBからAI要約一覧を取得する
func (s *GitHubService) ListRepoSummaries(userID uint) ([]models.GitHubRepoSummary, error) {
	return s.githubRepo.ListRepoSummaries(userID)
}

// SummarizeRepo リポジトリのREADMEをAIが解析し、技術的強みを要約する。
// キャッシュがあれば再生成しない。forceRefresh=trueで強制再生成。
func (s *GitHubService) SummarizeRepo(ctx context.Context, userID uint, fullName string, forceRefresh bool) (*models.GitHubRepoSummary, error) {
	// キャッシュ確認
	if !forceRefresh {
		cached, err := s.githubRepo.GetRepoSummary(userID, fullName)
		if err != nil {
			return nil, err
		}
		if cached != nil {
			return cached, nil
		}
	}

	profile, err := s.githubRepo.GetProfile(userID)
	if err != nil || profile == nil {
		return nil, fmt.Errorf("github profile not found")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// README取得
	readme, err := s.fetchREADME(ctx, client, profile.AccessToken, fullName)
	if err != nil {
		log.Printf("[SummarizeRepo] README fetch warning for %s: %v", fullName, err)
		readme = ""
	}

	// AI要約生成
	summary, err := s.generateRepoSummary(ctx, fullName, readme)
	if err != nil {
		return nil, fmt.Errorf("AI summary generation failed: %w", err)
	}

	summary.UserID = userID
	// DBに保存
	if err := s.githubRepo.UpsertRepoSummary(summary); err != nil {
		return nil, fmt.Errorf("save summary: %w", err)
	}
	return summary, nil
}

// fetchREADME GitHub API経由でリポジトリのREADMEを取得する
func (s *GitHubService) fetchREADME(ctx context.Context, client *http.Client, token, fullName string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/readme", githubAPIBase, fullName)
	body, err := s.doGet(ctx, client, token, url)
	if err != nil {
		return "", err
	}
	var resp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if resp.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(resp.Content, "\n", ""))
		if err != nil {
			return "", err
		}
		text := string(decoded)
		// 長すぎる場合は先頭4000文字に絞る
		if len(text) > 4000 {
			text = text[:4000]
		}
		return text, nil
	}
	return resp.Content, nil
}

// generateRepoSummary OpenAIを使ってリポジトリの技術的強みを要約する
func (s *GitHubService) generateRepoSummary(ctx context.Context, fullName, readme string) (*models.GitHubRepoSummary, error) {
	if s.openaiClient == nil {
		return nil, fmt.Errorf("openai client not configured")
	}

	readmeSection := "（READMEなし）"
	if readme != "" {
		readmeSection = readme
	}

	systemPrompt := `あなたはエンジニア採用のキャリアアドバイザーです。
GitHubリポジトリのREADMEを読み、技術的な強みを簡潔にまとめてください。JSONのみで返してください。`

	userPrompt := fmt.Sprintf(`以下のGitHubリポジトリ「%s」のREADMEを読み、エンジニア採用担当者に伝わる技術的強みを3点でまとめてください。

## README
%s

## 出力フォーマット（このキーと型を厳守）
{
  "summary_text": "技術的な強みの3行要約（全体を1段落で簡潔に）",
  "tech_reason": "技術選定の理由（なぜその技術・言語・フレームワークを選んだか）",
  "challenge": "解決した課題（どんな問題に取り組み、どう解決したか）",
  "achievement": "成果（数値・具体的な改善・学んだこと）"
}

※ 情報が不足している場合はREADMEから推測して記述してください。各フィールドは1〜2文で簡潔に。`, fullName, readmeSection)

	raw, err := s.openaiClient.ChatCompletionJSON(ctx, systemPrompt, userPrompt, 0.5, 800)
	if err != nil {
		return nil, err
	}

	cleaned := extractRepoSummaryJSON(raw)
	var payload struct {
		SummaryText string `json:"summary_text"`
		TechReason  string `json:"tech_reason"`
		Challenge   string `json:"challenge"`
		Achievement string `json:"achievement"`
	}
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return nil, fmt.Errorf("parse summary json: %w", err)
	}

	// FullName からユーザーIDは呼び出し元で設定するため0を仮置き
	return &models.GitHubRepoSummary{
		FullName:    fullName,
		SummaryText: payload.SummaryText,
		TechReason:  payload.TechReason,
		Challenge:   payload.Challenge,
		Achievement: payload.Achievement,
	}, nil
}

// extractRepoSummaryJSON マークダウンコードフェンスを除去してJSONを抽出する
func extractRepoSummaryJSON(raw string) string {
	s := strings.TrimSpace(raw)
	if start := strings.Index(s, "{"); start > 0 {
		s = s[start:]
	}
	if end := strings.LastIndex(s, "}"); end >= 0 && end < len(s)-1 {
		s = s[:end+1]
	}
	return s
}

// --- 内部ヘルパー ---

// githubAPIRepository GitHub API レスポンスのリポジトリ構造体
type githubAPIRepository struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	Language    string `json:"language"`
	StargazersCount int `json:"stargazers_count"`
	ForksCount  int    `json:"forks_count"`
	Fork        bool   `json:"fork"`
	UpdatedAt   string `json:"updated_at"`
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
					Name        string `json:"name"`
					NameWithOwner string `json:"nameWithOwner"`
					Description string `json:"description"`
					PrimaryLanguage *struct {
						Name string `json:"name"`
					} `json:"primaryLanguage"`
					StargazerCount int  `json:"stargazerCount"`
					ForkCount      int  `json:"forkCount"`
					IsFork         bool `json:"isFork"`
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
			Query     string                `json:"query"`
			Variables contributedReposVars  `json:"variables"`
		}{
			Query: `query($after: String) { viewer { repositoriesContributedTo(first: 100, includeUserRepositories: true, contributionTypes: [COMMIT, PULL_REQUEST, REPOSITORY], after: $after) { nodes { name nameWithOwner description primaryLanguage { name } stargazerCount forkCount isFork updatedAt } pageInfo { hasNextPage endCursor } } } }`,
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
