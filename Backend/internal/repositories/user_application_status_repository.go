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

// FindAll 条件を指定して応募一覧を取得する（管理者向け。§10.5）。
// userID/companyID が 0、status が空文字の場合はその条件で絞り込まない。
func (r *UserApplicationStatusRepository) FindAll(userID, companyID uint, status string) ([]*entity.UserApplicationStatus, error) {
	q := r.db.Preload("Company")
	if userID != 0 {
		q = q.Where("user_id = ?", userID)
	}
	if companyID != 0 {
		q = q.Where("company_id = ?", companyID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var ms []*models.UserApplicationStatus
	if err := q.Order("created_at DESC").Find(&ms).Error; err != nil {
		return nil, err
	}
	result := make([]*entity.UserApplicationStatus, len(ms))
	for i, m := range ms {
		result[i] = mapper.UserApplicationStatusToEntity(m)
	}
	return result, nil
}

// UpdateStatus 選考ステータスを更新する。notes が nil ならメモは変更しない（#1084）。
func (r *UserApplicationStatusRepository) UpdateStatus(id uint, status string, notes *string) error {
	now := time.Now()
	updates := map[string]any{
		"status":            status,
		"status_updated_at": now,
		"updated_at":        now,
	}
	if notes != nil {
		updates["notes"] = *notes
	}
	return r.db.Model(&models.UserApplicationStatus{}).
		Where("id = ?", id).
		Updates(updates).Error
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

// LowMatchApplication は「マッチ度が低いのに応募中」の1件（#1028）。
type LowMatchApplication struct {
	UserID      uint    `json:"-"`
	CompanyName string  `json:"company_name"`
	MatchScore  float64 `json:"match_score"`
	Status      string  `json:"status"`
}

// 進行中とみなさない応募ステータス。
// 終了済みの応募まで拾うと「今フォローすべき生徒」が埋もれる。
var terminalApplicationStatuses = []string{"withdrawn", "rejected", "not_applied"}

// FindLowMatchApplicationsByUsers は複数生徒の「低マッチのまま進行中の応募」を1クエリで返す（#1028）。
//
// 教員向け一覧は担当生徒ぶん繰り返し引くため、生徒ごとに問い合わせると
// 即 N+1 になる（FindLatestScoresByUsers と同じ理由）。
//
// threshold 未満のみを対象とし、境界値ちょうどは含めない
// （学生側の needsLowMatchConfirm と揃える。片方だけ <= にすると
// 「学生には確認が出ないのに教員一覧には出る」という食い違いが起きる）。
func (r *UserApplicationStatusRepository) FindLowMatchApplicationsByUsers(
	userIDs []uint, threshold float64,
) (map[uint][]LowMatchApplication, error) {
	result := map[uint][]LowMatchApplication{}
	if len(userIDs) == 0 {
		return result, nil
	}

	var rows []LowMatchApplication
	err := r.db.Table("user_application_statuses AS a").
		Select("a.user_id AS user_id, c.name AS company_name, m.match_score AS match_score, a.status AS status").
		Joins("JOIN user_company_matches m ON m.id = a.match_id").
		Joins("JOIN companies c ON c.id = a.company_id").
		Where("a.user_id IN ?", userIDs).
		Where("a.status NOT IN ?", terminalApplicationStatuses).
		Where("m.match_score < ?", threshold).
		Order("m.match_score ASC, a.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row)
	}
	return result, nil
}
