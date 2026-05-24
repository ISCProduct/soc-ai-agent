package repositories

import (
	"Backend/domain/entity"
	"Backend/domain/mapper"
	"Backend/internal/models"
	"time"

	"gorm.io/gorm"
)

type UserApplicationStatusRepository struct {
	db *gorm.DB
}

func NewUserApplicationStatusRepository(db *gorm.DB) *UserApplicationStatusRepository {
	return &UserApplicationStatusRepository{db: db}
}

// Create 応募ステータスを新規作成
func (r *UserApplicationStatusRepository) Create(app *entity.UserApplicationStatus) error {
	m := mapper.UserApplicationStatusFromEntity(app)
	if err := r.db.Create(m).Error; err != nil {
		return err
	}
	app.ID = m.ID
	app.CreatedAt = m.CreatedAt
	app.UpdatedAt = m.UpdatedAt
	return nil
}

// FindByUserAndCompany ユーザーIDと企業IDで検索（重複チェック用）
func (r *UserApplicationStatusRepository) FindByUserAndCompany(userID, companyID uint) (*entity.UserApplicationStatus, error) {
	var m models.UserApplicationStatus
	err := r.db.Where("user_id = ? AND company_id = ?", userID, companyID).
		Preload("Company").
		First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return mapper.UserApplicationStatusToEntity(&m), nil
}

// FindByID IDで取得
func (r *UserApplicationStatusRepository) FindByID(id uint) (*entity.UserApplicationStatus, error) {
	var m models.UserApplicationStatus
	err := r.db.Preload("Company").First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return mapper.UserApplicationStatusToEntity(&m), nil
}

// FindByUserID ユーザーの全応募一覧を取得
func (r *UserApplicationStatusRepository) FindByUserID(userID uint) ([]*entity.UserApplicationStatus, error) {
	var ms []*models.UserApplicationStatus
	err := r.db.Where("user_id = ?", userID).
		Preload("Company").
		Order("created_at DESC").
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	result := make([]*entity.UserApplicationStatus, len(ms))
	for i, m := range ms {
		result[i] = mapper.UserApplicationStatusToEntity(m)
	}
	return result, nil
}

// UpdateStatus 選考ステータスを更新
func (r *UserApplicationStatusRepository) UpdateStatus(id uint, status, notes string) error {
	now := time.Now()
	return r.db.Model(&models.UserApplicationStatus{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":            status,
			"notes":             notes,
			"status_updated_at": now,
			"updated_at":        now,
		}).Error
}

// GetCorrelationByCompany 企業ごとのマッチングスコア×選考通過率の相関データを取得
func (r *UserApplicationStatusRepository) GetCorrelationByCompany(companyID uint) ([]map[string]any, error) {
	type Row struct {
		MatchScore float64
		Status     string
	}
	var rows []Row
	err := r.db.Table("user_application_statuses uas").
		Select("ucm.match_score, uas.status").
		Joins("JOIN user_company_matches ucm ON ucm.id = uas.match_id").
		Where("uas.company_id = ?", companyID).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(rows))
	for i, row := range rows {
		result[i] = map[string]any{
			"match_score": row.MatchScore,
			"status":      row.Status,
		}
	}
	return result, nil
}

// GetGlobalCorrelation 全企業横断のマッチングスコア×選考結果相関データ
func (r *UserApplicationStatusRepository) GetGlobalCorrelation() ([]map[string]any, error) {
	type Row struct {
		CompanyID  uint
		MatchScore float64
		Status     string
	}
	var rows []Row
	err := r.db.Table("user_application_statuses uas").
		Select("uas.company_id, ucm.match_score, uas.status").
		Joins("JOIN user_company_matches ucm ON ucm.id = uas.match_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, len(rows))
	for i, row := range rows {
		result[i] = map[string]any{
			"company_id":  row.CompanyID,
			"match_score": row.MatchScore,
			"status":      row.Status,
		}
	}
	return result, nil
}
