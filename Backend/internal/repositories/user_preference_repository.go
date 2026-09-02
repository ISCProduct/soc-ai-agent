package repositories

import (
	"strings"

	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserPreferenceRepository は学生の希望条件を扱う（#1094）。
type UserPreferenceRepository struct {
	db *gorm.DB
}

func NewUserPreferenceRepository(db *gorm.DB) *UserPreferenceRepository {
	return &UserPreferenceRepository{db: db}
}

// FindByUserID は希望条件を返す。未設定なら nil, nil。
func (r *UserPreferenceRepository) FindByUserID(userID uint) (*models.UserPreference, error) {
	var m models.UserPreference
	err := r.db.Where("user_id = ?", userID).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Upsert は希望条件を作成または更新する（1学生1レコード）。
func (r *UserPreferenceRepository) Upsert(pref *models.UserPreference) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"desired_industry_id", "desired_job_category_id", "desired_location", "note", "updated_at",
		}),
	}).Create(pref).Error
}

// SetScoutVisibility はスカウト公開同意フラグのみを更新する（#1094）。
// entity 全体を保存すると他フィールドを巻き戻す恐れがあるため、対象カラムだけを更新する。
func (r *UserPreferenceRepository) SetScoutVisibility(userID uint, allow bool) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Update("allow_scout_visibility", allow).Error
}

// GetScoutVisibility は現在のスカウト公開同意フラグを返す。
func (r *UserPreferenceRepository) GetScoutVisibility(userID uint) (bool, error) {
	var allow bool
	err := r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Select("allow_scout_visibility").Scan(&allow).Error
	return allow, err
}

// ScoutProfileText は同意済み学生のベクトル化対象テキストを組み立てる（#1094）。
// 対象は学生が自ら入力した資格・希望条件のみ。履歴書全文や面接発話は
// 公開同意の範囲が別途整理されるまで含めない。
func (r *UserPreferenceRepository) ScoutProfileText(userID uint) (string, error) {
	var row struct {
		CertificationsAcquired   string
		CertificationsInProgress string
		DesiredLocation          string
		Note                     string
		IndustryName             string
	}
	err := r.db.Table("users AS u").
		Joins("LEFT JOIN user_preferences AS up ON up.user_id = u.id").
		Joins("LEFT JOIN industries AS i ON i.id = up.desired_industry_id").
		Where("u.id = ?", userID).
		Select(`u.certifications_acquired, u.certifications_in_progress,
			COALESCE(up.desired_location, '') AS desired_location,
			COALESCE(up.note, '') AS note,
			COALESCE(i.name, '') AS industry_name`).
		Scan(&row).Error
	if err != nil {
		return "", err
	}
	parts := []string{}
	for _, p := range [][2]string{
		{"取得資格", row.CertificationsAcquired},
		{"勉強中の資格", row.CertificationsInProgress},
		{"希望業界", row.IndustryName},
		{"希望勤務地", row.DesiredLocation},
		{"自己PR", row.Note},
	} {
		if v := strings.TrimSpace(p[1]); v != "" {
			parts = append(parts, p[0]+": "+v)
		}
	}
	return strings.Join(parts, "\n"), nil
}
