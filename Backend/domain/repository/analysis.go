package repository

import "Backend/domain/entity"

// UserWeightScoreRepository はユーザースコアの永続化インターフェース。
type UserWeightScoreRepository interface {
	// SetScore は新規レコードをスコアの絶対値で作成する
	SetScore(userID uint, sessionID, category string, absoluteScore int) error
	// AddScore は既存レコードのスコアに差分を加算する
	AddScore(userID uint, sessionID, category string, delta int) error
	FindByUserAndSession(userID uint, sessionID string) ([]entity.UserWeightScore, error)
	FindTopCategories(userID uint, sessionID string, limit int) ([]entity.UserWeightScore, error)
	FindByUserSessionAndCategory(userID uint, sessionID, category string) (*entity.UserWeightScore, error)
	CountByUserAndSession(userID uint, sessionID string) (int64, error)
}

// AnalysisPhaseRepository は分析フェーズ定義の永続化インターフェース。
type AnalysisPhaseRepository interface {
	FindAll() ([]entity.AnalysisPhase, error)
	FindByID(id uint) (*entity.AnalysisPhase, error)
	FindByName(name string) (*entity.AnalysisPhase, error)
}

// UserAnalysisProgressRepository はユーザーの分析進捗の永続化インターフェース。
type UserAnalysisProgressRepository interface {
	FindByUserAndSession(userID uint, sessionID string) ([]entity.UserAnalysisProgress, error)
	FindOrCreate(userID uint, sessionID string, phaseID uint) (*entity.UserAnalysisProgress, error)
	Update(progress *entity.UserAnalysisProgress) error
	GetCurrentPhase(userID uint, sessionID string) (*entity.UserAnalysisProgress, error)
}

// UserCompanyMatchRepository はユーザーと企業のマッチング結果の永続化インターフェース。
type UserCompanyMatchRepository interface {
	CreateOrUpdate(match *entity.UserCompanyMatch) error
	FindTopMatchesByUserAndSession(userID uint, sessionID string, limit int) ([]*entity.UserCompanyMatch, error)
	FindByID(id uint) (*entity.UserCompanyMatch, error)
	MarkAsViewed(matchID uint) error
	ToggleFavorite(matchID uint) error
	MarkAsApplied(matchID uint) error
	FindFavoritesByUser(userID uint, sessionID string) ([]*entity.UserCompanyMatch, error)
	GetMatchStatistics(userID uint, sessionID string) (map[string]interface{}, error)
}
