package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
)

type AnalysisPhaseRepository struct {
	db *gorm.DB
}

func NewAnalysisPhaseRepository(db *gorm.DB) *AnalysisPhaseRepository {
	return &AnalysisPhaseRepository{db: db}
}

// FindAll 全フェーズを順序順に取得
func (r *AnalysisPhaseRepository) FindAll() ([]models.AnalysisPhase, error) {
	var ms []models.AnalysisPhase
	err := r.db.Order("phase_order ASC").Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return ms, nil
}

// FindByID IDでフェーズを取得
func (r *AnalysisPhaseRepository) FindByID(id uint) (*models.AnalysisPhase, error) {
	var m models.AnalysisPhase
	err := r.db.First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// FindByName 名前でフェーズを取得
func (r *AnalysisPhaseRepository) FindByName(name string) (*models.AnalysisPhase, error) {
	var m models.AnalysisPhase
	err := r.db.Where("phase_name = ?", name).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

type UserAnalysisProgressRepository struct {
	db *gorm.DB
}

func NewUserAnalysisProgressRepository(db *gorm.DB) *UserAnalysisProgressRepository {
	return &UserAnalysisProgressRepository{db: db}
}

// FindByUserAndSession ユーザーとセッションで進捗を取得
func (r *UserAnalysisProgressRepository) FindByUserAndSession(userID uint, sessionID string) ([]models.UserAnalysisProgress, error) {
	var ms []models.UserAnalysisProgress
	err := r.db.Preload("Phase").
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Order("phase_id ASC").
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return ms, nil
}

// FindOrCreate フェーズの進捗を取得または作成
func (r *UserAnalysisProgressRepository) FindOrCreate(userID uint, sessionID string, phaseID uint) (*models.UserAnalysisProgress, error) {
	var m models.UserAnalysisProgress
	err := r.db.Where("user_id = ? AND session_id = ? AND phase_id = ?", userID, sessionID, phaseID).
		FirstOrCreate(&m, models.UserAnalysisProgress{
			UserID:    userID,
			SessionID: sessionID,
			PhaseID:   phaseID,
		}).Error
	if err != nil {
		return nil, err
	}
	// Phaseをロード
	r.db.Preload("Phase").First(&m, m.ID)
	return &m, nil
}

// Update 進捗を更新
func (r *UserAnalysisProgressRepository) Update(progress *models.UserAnalysisProgress) error {
	// Preload 済みの Phase は書き込まない(以前の変換でも Phase は書き込み対象外だった)。
	return r.db.Omit("Phase").Save(progress).Error
}
