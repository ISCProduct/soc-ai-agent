package repositories

import (
	"Backend/internal/models"
	"strings"

	"gorm.io/gorm"
)

// StudentSearchFilters は企業向け学生検索の絞り込み条件（#1094）。
// ゼロ値のフィールドは条件に含めない（AND結合）。
type StudentSearchFilters struct {
	IndustryID uint   // 希望業界。親業界を指定した場合は直下の子業界も含める
	Location   string // 希望勤務地（部分一致）
	Skill      string // 取得資格・勉強中の資格（部分一致）
	Tag        string // 自社タグ（完全一致）
	UserIDs    []uint // セマンティック検索の結果で絞る場合のみ指定
	Limit      int
	Offset     int
}

// StudentSearchRow は学生一覧の1行。企業に公開してよい項目のみを持つ（メールアドレス等は含めない）。
type StudentSearchRow struct {
	UserID                   uint   `gorm:"column:user_id" json:"user_id"`
	Name                     string `gorm:"column:name" json:"name"`
	SchoolName               string `gorm:"column:school_name" json:"school_name"`
	TargetLevel              string `gorm:"column:target_level" json:"target_level"`
	AvatarURL                string `gorm:"column:avatar_url" json:"avatar_url"`
	CertificationsAcquired   string `gorm:"column:certifications_acquired" json:"certifications_acquired"`
	CertificationsInProgress string `gorm:"column:certifications_in_progress" json:"certifications_in_progress"`
	DesiredLocation          string `gorm:"column:desired_location" json:"desired_location"`
	DesiredIndustryID        *uint  `gorm:"column:desired_industry_id" json:"desired_industry_id,omitempty"`
	DesiredIndustryName      string `gorm:"column:desired_industry_name" json:"desired_industry_name"`
}

type StudentSearchRepository struct {
	db *gorm.DB
}

func NewStudentSearchRepository(db *gorm.DB) *StudentSearchRepository {
	return &StudentSearchRepository{db: db}
}

const defaultStudentSearchLimit = 30
const maxStudentSearchLimit = 100

// visibleStudents は「企業に公開してよい学生」の基本条件を組み立てる。
// スカウト公開に同意済み・未退会・学生ロール・非ゲストのみを対象とし、
// 全ての検索経路（一覧/セマンティック検索/詳細）がこの条件を通る。
func (r *StudentSearchRepository) visibleStudents(companyID uint, f StudentSearchFilters) *gorm.DB {
	q := r.db.Table("users AS u").
		Joins("LEFT JOIN user_preferences AS up ON up.user_id = u.id").
		Joins("LEFT JOIN industries AS i ON i.id = up.desired_industry_id").
		Where("u.allow_scout_visibility = ?", true).
		Where("u.withdrawn_at IS NULL").
		Where("u.role = ?", "student").
		Where("u.is_guest = ?", false)

	if f.IndustryID > 0 {
		// 親業界が指定された場合は直下の子業界の希望者も拾う。
		q = q.Where("up.desired_industry_id IN (?)",
			r.db.Table("industries").Select("id").
				Where("id = ? OR parent_id = ?", f.IndustryID, f.IndustryID))
	}
	if s := strings.TrimSpace(f.Location); s != "" {
		q = q.Where("up.desired_location LIKE ?", "%"+escapeLike(s)+"%")
	}
	if s := strings.TrimSpace(f.Skill); s != "" {
		like := "%" + escapeLike(s) + "%"
		q = q.Where("(u.certifications_acquired LIKE ? OR u.certifications_in_progress LIKE ?)", like, like)
	}
	if s := strings.TrimSpace(f.Tag); s != "" {
		// タグは必ず自社(company_id)に限定する。他社のタグ付けは条件に一致しない。
		q = q.Where("EXISTS (?)",
			r.db.Table("company_student_tags AS t").Select("1").
				Where("t.user_id = u.id AND t.company_id = ? AND t.tag_name = ?", companyID, s))
	}
	if f.UserIDs != nil {
		// セマンティック検索の結果が0件のときは空スライスが渡る。
		// その場合 IN () は不正なので、確実に0件になる条件へ落とす。
		if len(f.UserIDs) == 0 {
			return q.Where("1 = 0")
		}
		q = q.Where("u.id IN ?", f.UserIDs)
	}
	return q
}

// Search はフィルタ条件に一致する学生一覧と総件数を返す。
func (r *StudentSearchRepository) Search(companyID uint, f StudentSearchFilters) ([]StudentSearchRow, int64, error) {
	var total int64
	if err := r.visibleStudents(companyID, f).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = defaultStudentSearchLimit
	}
	if limit > maxStudentSearchLimit {
		limit = maxStudentSearchLimit
	}
	offset := max(f.Offset, 0)

	rows := []StudentSearchRow{}
	err := r.visibleStudents(companyID, f).
		Select(`u.id AS user_id, u.name, u.school_name, u.target_level, u.avatar_url,
			u.certifications_acquired, u.certifications_in_progress,
			COALESCE(up.desired_location, '') AS desired_location,
			up.desired_industry_id,
			COALESCE(i.name, '') AS desired_industry_name`).
		Order("u.id DESC").
		Limit(limit).Offset(offset).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// IsVisible は対象学生が企業に公開されているかを返す（詳細画面の認可に使う）。
func (r *StudentSearchRepository) IsVisible(userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).
		Where("id = ? AND allow_scout_visibility = ? AND withdrawn_at IS NULL AND role = ? AND is_guest = ?",
			userID, true, "student", false).
		Count(&count).Error
	return count > 0, err
}

// escapeLike は LIKE のワイルドカードをエスケープし、利用者入力で全件一致するのを防ぐ。
func escapeLike(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_")
	return r.Replace(s)
}
