package repositories

import (
	"Backend/internal/models"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GitHubRepository GitHub連携データのDB操作
type GitHubRepository struct {
	db *gorm.DB
}

func NewGitHubRepository(db *gorm.DB) *GitHubRepository {
	return &GitHubRepository{db: db}
}

// UpsertProfile GitHubプロフィールを保存/更新
func (r *GitHubRepository) UpsertProfile(profile *models.GitHubProfile) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"git_hub_login", "access_token", "total_contributions", "public_repos", "followers", "following", "synced_at", "updated_at"}),
	}).Create(profile).Error
}

// GetProfile ユーザーIDでGitHubプロフィールを取得
func (r *GitHubRepository) GetProfile(userID uint) (*models.GitHubProfile, error) {
	var profile models.GitHubProfile
	if err := r.db.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &profile, nil
}

// ReplaceRepositories ユーザーのリポジトリ一覧を全件置換
func (r *GitHubRepository) ReplaceRepositories(userID uint, repos []models.GitHubRepo) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.GitHubRepo{}).Error; err != nil {
			return err
		}
		if len(repos) == 0 {
			return nil
		}
		return tx.Create(&repos).Error
	})
}

// GetRepositories ユーザーのリポジトリ一覧を取得（star数降順）
func (r *GitHubRepository) GetRepositories(userID uint) ([]models.GitHubRepo, error) {
	var repos []models.GitHubRepo
	if err := r.db.Where("user_id = ?", userID).Order("stars desc").Find(&repos).Error; err != nil {
		return nil, err
	}
	return repos, nil
}

// ReplaceLanguageStats ユーザーの言語使用比率を全件置換
func (r *GitHubRepository) ReplaceLanguageStats(userID uint, stats []models.GitHubLanguageStat) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.GitHubLanguageStat{}).Error; err != nil {
			return err
		}
		if len(stats) == 0 {
			return nil
		}
		return tx.Create(&stats).Error
	})
}

// GetLanguageStats ユーザーの言語使用比率を取得（使用量降順）
func (r *GitHubRepository) GetLanguageStats(userID uint) ([]models.GitHubLanguageStat, error) {
	var stats []models.GitHubLanguageStat
	if err := r.db.Where("user_id = ?", userID).Order("bytes desc").Find(&stats).Error; err != nil {
		return nil, err
	}
	return stats, nil
}

// UpdateSyncedAt 最終同期日時を更新
func (r *GitHubRepository) UpdateSyncedAt(userID uint, t time.Time) error {
	return r.db.Model(&models.GitHubProfile{}).
		Where("user_id = ?", userID).
		Update("synced_at", t).Error
}

// GetRepoSummary ユーザーID+リポジトリ名でAI要約を取得
func (r *GitHubRepository) GetRepoSummary(userID uint, fullName string) (*models.GitHubRepoSummary, error) {
	var s models.GitHubRepoSummary
	if err := r.db.Where("user_id = ? AND full_name = ?", userID, fullName).First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// ListRepoSummaries ユーザーのAI要約のうち、promptVersion の版で作られたものを取得する。
// 古い版の要約は注入で作られた虚偽の実績を含みうるので、一覧にも出さない（#2185）。
func (r *GitHubRepository) ListRepoSummaries(userID uint, promptVersion string) ([]models.GitHubRepoSummary, error) {
	var summaries []models.GitHubRepoSummary
	if err := r.db.Where("user_id = ? AND prompt_version = ?", userID, promptVersion).Order("updated_at desc").Find(&summaries).Error; err != nil {
		return nil, err
	}
	return summaries, nil
}

// UpsertRepoSummary AI要約を保存/更新
func (r *GitHubRepository) UpsertRepoSummary(s *models.GitHubRepoSummary) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "full_name"}},
		// prompt_version も更新しないと、作り直した後も古い版のままになり、毎回作り直してしまう（#2185）
		DoUpdates: clause.AssignmentColumns([]string{"summary_text", "tech_reason", "challenge", "achievement", "prompt_version", "updated_at"}),
	}).Create(s).Error
}
