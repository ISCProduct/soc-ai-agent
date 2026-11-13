package repository

import (
	"Backend/internal/models"
)

// UserWeightScoreRepository はユーザースコアの永続化インターフェース。
type UserWeightScoreRepository interface {
	// SetScore は新規レコードをスコアの絶対値で作成する
	SetScore(userID uint, sessionID, category string, absoluteScore int) error
	// AddScore は既存レコードのスコアに差分を加算する
	AddScore(userID uint, sessionID, category string, delta int) error
	// SetScoreWithPromptVersion は SetScore に加えて、評価に使ったプロンプト版を記録する
	SetScoreWithPromptVersion(userID uint, sessionID, category string, absoluteScore int, promptVersion string) error
	// AddScoreWithPromptVersion は AddScore に加えて、評価に使ったプロンプト版を記録する
	AddScoreWithPromptVersion(userID uint, sessionID, category string, delta int, promptVersion string) error
	FindByUserAndSession(userID uint, sessionID string) ([]models.UserWeightScore, error)
	FindTopCategories(userID uint, sessionID string, limit int) ([]models.UserWeightScore, error)
	FindByUserSessionAndCategory(userID uint, sessionID, category string) (*models.UserWeightScore, error)
	CountByUserAndSession(userID uint, sessionID string) (int64, error)
}

// AnalysisPhaseRepository は分析フェーズ定義の永続化インターフェース。
type AnalysisPhaseRepository interface {
	FindAll() ([]models.AnalysisPhase, error)
	FindByID(id uint) (*models.AnalysisPhase, error)
	FindByName(name string) (*models.AnalysisPhase, error)
}

// UserAnalysisProgressRepository はユーザーの分析進捗の永続化インターフェース。
type UserAnalysisProgressRepository interface {
	FindByUserAndSession(userID uint, sessionID string) ([]models.UserAnalysisProgress, error)
	FindOrCreate(userID uint, sessionID string, phaseID uint) (*models.UserAnalysisProgress, error)
	Update(progress *models.UserAnalysisProgress) error
}

// UserCompanyMatchRepository はユーザーと企業のマッチング結果の永続化インターフェース。
type UserCompanyMatchRepository interface {
	CreateOrUpdate(match *models.UserCompanyMatch) error
	// CreateOrUpdateBatch は同一 user/session のマッチ結果を一括 upsert する（既存のお気に入り等フラグは保持）。
	CreateOrUpdateBatch(matches []*models.UserCompanyMatch) (int, error)
	FindTopMatchesByUserAndSession(userID uint, sessionID string, limit int) ([]*models.UserCompanyMatch, error)
	FindByID(id uint) (*models.UserCompanyMatch, error)
	MarkAsViewed(matchID uint) error
	ToggleFavorite(matchID uint) error
	MarkAsApplied(matchID uint) error
	GetMatchStatistics(userID uint, sessionID string) (map[string]any, error)
}
