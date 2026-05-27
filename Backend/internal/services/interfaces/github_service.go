package interfaces

import (
	"Backend/internal/models"
	"context"
)

type GitHubService interface {
	GetProfile(userID uint) (*models.GitHubProfile, error)
	GetRepositories(userID uint) ([]models.GitHubRepo, error)
	GetLanguageStats(userID uint) ([]models.GitHubLanguageStat, error)
	TriggerAsyncSync(userID uint, force bool)
	SyncUserData(ctx context.Context, userID uint, force bool) error
	ListRepoSummaries(userID uint) ([]models.GitHubRepoSummary, error)
	SummarizeRepo(ctx context.Context, userID uint, fullName string, forceRefresh bool) (*models.GitHubRepoSummary, error)
}

type SkillScoreService interface {
	GetScores(userID uint) ([]models.SkillScore, error)
}
