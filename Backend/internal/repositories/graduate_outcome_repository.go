package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// GraduateOutcomeRepository は学生向けの卒業生の進路実績（#2032）を、学校で絞って集計する。
//
// 卒業生の氏名・学科・メモ（graduate_name / department / note）は読まない。学生に出すのは
// 企業・年度・職種ごとの人数だけ。どのクエリも school_id で絞る（他校の実績は出さない）。
// 企業は学生向けの企業表示と同じ条件（有効・未削除・審査前のゲスト投稿でない）で絞る。
type GraduateOutcomeRepository struct {
	db *gorm.DB
}

func NewGraduateOutcomeRepository(db *gorm.DB) *GraduateOutcomeRepository {
	return &GraduateOutcomeRepository{db: db}
}

// ErrGraduateOutcomeUserNotFound は学生が存在しない（退会済みを含む）ことを表す。
var ErrGraduateOutcomeUserNotFound = errors.New("graduate outcome: user not found")

// StudentSchoolID は学生の個別校の ID を返す。未登録なら nil。
//
// users.school_id をそのまま信用せず、学校の学園が本人の学園と一致するときだけ返す
// （一致しない・学校が消えている場合は nil = 実績を出さない）。AssessmentRepository.FindUserSchoolID
// （#1675）と同じ fail-close。学生が存在しなければ ErrGraduateOutcomeUserNotFound。
func (r *GraduateOutcomeRepository) StudentSchoolID(ctx context.Context, userID uint) (*uint, error) {
	var u struct {
		SchoolID       *uint
		OrganizationID uint
	}
	err := r.db.WithContext(ctx).Table("users").Select("school_id", "organization_id").
		Where("id = ? AND withdrawn_at IS NULL", userID).Take(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGraduateOutcomeUserNotFound
	}
	if err != nil {
		return nil, err
	}
	if u.SchoolID == nil {
		return nil, nil
	}
	var sc struct{ OrganizationID uint }
	err = r.db.WithContext(ctx).Table("schools").Select("organization_id").Where("id = ?", *u.SchoolID).Take(&sc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if sc.OrganizationID != u.OrganizationID {
		return nil, nil
	}
	return u.SchoolID, nil
}

// visibleCompany は学生に見せてよい企業の条件（companies を c として JOIN している前提）。
// 無効・削除済みの企業と、審査前のゲスト投稿企業（guestEntryVisibilityGuard と同じ判定）を除く。
const visibleCompany = `c.is_active = true AND c.deleted_at IS NULL AND NOT (
	c.data_status <> 'published'
	AND (c.is_guest_entry = true OR EXISTS (SELECT 1 FROM company_entry_submissions s WHERE s.company_id = c.id))
)`

// CompanyVisible は企業が存在し、学生に見せてよいかを返す。
func (r *GraduateOutcomeRepository) CompanyVisible(ctx context.Context, companyID uint) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("companies c").Where("c.id = ?", companyID).Where(visibleCompany).Count(&n).Error
	return n > 0, err
}

// GraduateOutcomeRow は企業・年度ごとの人数。年度が未登録（0・NULL）なら GraduationYear は 0。
type GraduateOutcomeRow struct {
	CompanyID      uint
	CompanyName    string
	Industry       string
	GraduationYear int
	Count          int
}

// GraduateOutcomeFilter は一覧の絞り込み。0・空は絞らない。
type GraduateOutcomeFilter struct {
	Year     int
	Industry string
}

func (r *GraduateOutcomeRepository) base(ctx context.Context, schoolID uint) *gorm.DB {
	return r.db.WithContext(ctx).Table("graduate_employments ge").
		Joins("JOIN companies c ON c.id = ge.company_id").
		Where("ge.school_id = ? AND ge.deleted_at IS NULL", schoolID).
		Where(visibleCompany)
}

const outcomeRowSelect = "ge.company_id AS company_id, c.name AS company_name, COALESCE(c.industry, '') AS industry, COALESCE(ge.graduation_year, 0) AS graduation_year, COUNT(*) AS count"
const outcomeRowGroup = "ge.company_id, c.name, c.industry, COALESCE(ge.graduation_year, 0)"

// CountsByCompanyYear は学校の卒業生の就職先を、企業・年度ごとの人数で返す。
func (r *GraduateOutcomeRepository) CountsByCompanyYear(ctx context.Context, schoolID uint, f GraduateOutcomeFilter) ([]GraduateOutcomeRow, error) {
	q := r.base(ctx, schoolID).Select(outcomeRowSelect)
	if f.Year > 0 {
		q = q.Where("ge.graduation_year = ?", f.Year)
	}
	if f.Industry != "" {
		q = q.Where("c.industry = ?", f.Industry)
	}
	var out []GraduateOutcomeRow
	if err := q.Group(outcomeRowGroup).Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// FilterOptions は学校の実績にある年度と業界を返す（絞り込みの選択肢）。年度が未登録の行は選択肢に出さない。
func (r *GraduateOutcomeRepository) FilterOptions(ctx context.Context, schoolID uint) ([]int, []string, error) {
	var years []int
	if err := r.base(ctx, schoolID).Where("ge.graduation_year > 0").Distinct().Order("ge.graduation_year DESC").Pluck("ge.graduation_year", &years).Error; err != nil {
		return nil, nil, err
	}
	var industries []string
	if err := r.base(ctx, schoolID).Where("c.industry <> ''").Distinct().Order("c.industry").Pluck("c.industry", &industries).Error; err != nil {
		return nil, nil, err
	}
	return years, industries, nil
}

// GraduatePositionCount は職種ごとの人数。職種が未登録（または別企業の職種を指している）なら Title は空。
type GraduatePositionCount struct {
	Title string
	Count int
}

// CompanyCounts は1社について、学校の卒業生の年度別と職種別の人数を返す。
func (r *GraduateOutcomeRepository) CompanyCounts(ctx context.Context, schoolID, companyID uint) ([]GraduateOutcomeRow, []GraduatePositionCount, error) {
	var byYear []GraduateOutcomeRow
	if err := r.base(ctx, schoolID).Where("ge.company_id = ?", companyID).
		Select(outcomeRowSelect).Group(outcomeRowGroup).
		Order("graduation_year DESC").Scan(&byYear).Error; err != nil {
		return nil, nil, err
	}
	var byPosition []GraduatePositionCount
	// 職種はその企業のものだけ結ぶ（別企業の求人 ID が入った行は「職種の登録なし」に寄せる）
	if err := r.base(ctx, schoolID).Where("ge.company_id = ?", companyID).
		Joins("LEFT JOIN company_job_positions jp ON jp.id = ge.job_position_id AND jp.company_id = ge.company_id").
		Select("COALESCE(jp.title, '') AS title, COUNT(*) AS count").
		Group("COALESCE(jp.title, '')").Order("count DESC, title").Scan(&byPosition).Error; err != nil {
		return nil, nil, err
	}
	return byYear, byPosition, nil
}
