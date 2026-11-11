package repositories

import (
	"Backend/domain/entity"
	"Backend/domain/mapper"
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
func (r *AnalysisPhaseRepository) FindAll() ([]entity.AnalysisPhase, error) {
	var ms []models.AnalysisPhase
	err := r.db.Order("phase_order ASC").Find(&ms).Error
	if err != nil {
		return nil, err
	}
	result := make([]entity.AnalysisPhase, len(ms))
	for i, m := range ms {
		e := mapper.AnalysisPhaseToEntity(&m)
		result[i] = *e
	}
	return result, nil
}

// FindByID IDでフェーズを取得
func (r *AnalysisPhaseRepository) FindByID(id uint) (*entity.AnalysisPhase, error) {
	var m models.AnalysisPhase
	err := r.db.First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return mapper.AnalysisPhaseToEntity(&m), nil
}

// FindByName 名前でフェーズを取得
func (r *AnalysisPhaseRepository) FindByName(name string) (*entity.AnalysisPhase, error) {
	var m models.AnalysisPhase
	err := r.db.Where("phase_name = ?", name).First(&m).Error
	if err != nil {
		return nil, err
	}
	return mapper.AnalysisPhaseToEntity(&m), nil
}

type UserAnalysisProgressRepository struct {
	db *gorm.DB
}

func NewUserAnalysisProgressRepository(db *gorm.DB) *UserAnalysisProgressRepository {
	return &UserAnalysisProgressRepository{db: db}
}

// FindByUserAndSession ユーザーとセッションで進捗を取得
func (r *UserAnalysisProgressRepository) FindByUserAndSession(userID uint, sessionID string) ([]entity.UserAnalysisProgress, error) {
	var ms []models.UserAnalysisProgress
	err := r.db.Preload("Phase").
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Order("phase_id ASC").
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	result := make([]entity.UserAnalysisProgress, len(ms))
	for i, m := range ms {
		e := mapper.UserAnalysisProgressToEntity(&m)
		result[i] = *e
	}
	return result, nil
}

// FindOrCreate フェーズの進捗を取得または作成
func (r *UserAnalysisProgressRepository) FindOrCreate(userID uint, sessionID string, phaseID uint) (*entity.UserAnalysisProgress, error) {
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
	return mapper.UserAnalysisProgressToEntity(&m), nil
}

// Update 進捗を更新
func (r *UserAnalysisProgressRepository) Update(progress *entity.UserAnalysisProgress) error {
	m := mapper.UserAnalysisProgressFromEntity(progress)
	return r.db.Save(m).Error
}
