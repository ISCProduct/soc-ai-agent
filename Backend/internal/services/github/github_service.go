package github

import (
	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/repositories"
	"Backend/internal/services/skillscore"
	"time"
)

const (
	githubAPIBase    = "https://api.github.com"
	githubGraphQLURL = "https://api.github.com/graphql"
	// レート制限: 最終同期から1時間以内は再同期しない
	syncCacheDuration = time.Hour
	// レート制限リトライ: 最大2回
	maxRetries                  = 2
	githubTokenEncryptionKeyEnv = "GITHUB_TOKEN_ENCRYPTION_KEY"
	// プロンプトバージョン（#457）
	repoSummaryPromptVersion = "v2"
)

// GitHubService GitHub API連携サービス
type GitHubService struct {
	githubRepo        *repositories.GitHubRepository
	skillScoreService *skillscore.SkillScoreService
	apiBaseURL        string // テスト用オーバーライド（空なら githubAPIBase を使用）
	graphQLURL        string // テスト用オーバーライド（空なら githubGraphQLURL を使用）
	openaiClient      *openai.Client
}

func NewGitHubService(githubRepo *repositories.GitHubRepository, skillScoreService *skillscore.SkillScoreService, openaiClient *openai.Client) *GitHubService {
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
