package mocks

import (
	"Backend/internal/models"
	"context"

	"github.com/stretchr/testify/mock"
)

type GitHubServiceMock struct {
	mock.Mock
}

func (m *GitHubServiceMock) GetProfile(userID uint) (*models.GitHubProfile, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GitHubProfile), args.Error(1)
}

func (m *GitHubServiceMock) GetRepositories(userID uint) ([]models.GitHubRepo, error) {
	args := m.Called(userID)
	return args.Get(0).([]models.GitHubRepo), args.Error(1)
}

func (m *GitHubServiceMock) GetLanguageStats(userID uint) ([]models.GitHubLanguageStat, error) {
	args := m.Called(userID)
	return args.Get(0).([]models.GitHubLanguageStat), args.Error(1)
}

func (m *GitHubServiceMock) TriggerAsyncSync(userID uint, force bool) {
	m.Called(userID, force)
}

func (m *GitHubServiceMock) SyncUserData(ctx context.Context, userID uint, force bool) error {
	return m.Called(ctx, userID, force).Error(0)
}

func (m *GitHubServiceMock) ListRepoSummaries(userID uint) ([]models.GitHubRepoSummary, error) {
	args := m.Called(userID)
	return args.Get(0).([]models.GitHubRepoSummary), args.Error(1)
}

func (m *GitHubServiceMock) SummarizeRepo(ctx context.Context, userID uint, fullName string, forceRefresh bool, targetRole string) (*models.GitHubRepoSummary, error) {
	args := m.Called(ctx, userID, fullName, forceRefresh, targetRole)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GitHubRepoSummary), args.Error(1)
}

type SkillScoreServiceMock struct {
	mock.Mock
}

func (m *SkillScoreServiceMock) GetScores(userID uint) ([]models.SkillScore, error) {
	args := m.Called(userID)
	return args.Get(0).([]models.SkillScore), args.Error(1)
}
