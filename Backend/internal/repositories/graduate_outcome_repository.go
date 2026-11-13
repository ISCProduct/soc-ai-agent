package repositories

import (
	"gorm.io/gorm"
)

// GraduateOutcomeRepository は学生向けの卒業生の進路実績（#2032）を、学校で絞って集計する。
//
// 卒業生の氏名・学科・メモ（graduate_name / department / note）は読まない。学生に出すのは
// 企業・年度・職種ごとの人数だけ。どのクエリも school_id で絞る（他校の実績は出さない）。
type GraduateOutcomeRepository struct {
	db *gorm.DB
}

func NewGraduateOutcomeRepository(db *gorm.DB) *GraduateOutcomeRepository {
	return &GraduateOutcomeRepository{db: db}
}

// StudentSchoolID は学生の個別校の ID を返す。未登録なら nil。
func (r *GraduateOutcomeRepository) StudentSchoolID(userID uint) (*uint, error) {
	var row struct{ SchoolID *uint }
	err := r.db.Table("users").Select("school_id").Where("id = ? AND withdrawn_at IS NULL", userID).Scan(&row).Error
	return row.SchoolID, err
}

// GraduateOutcomeRow は企業・年度ごとの人数。
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

func (r *GraduateOutcomeRepository) base(schoolID uint) *gorm.DB {
	return r.db.Table("graduate_employments ge").
		Joins("JOIN companies c ON c.id = ge.company_id").
		Where("ge.school_id = ? AND ge.deleted_at IS NULL", schoolID)
}

// CountsByCompanyYear は学校の卒業生の就職先を、企業・年度ごとの人数で返す。
func (r *GraduateOutcomeRepository) CountsByCompanyYear(schoolID uint, f GraduateOutcomeFilter) ([]GraduateOutcomeRow, error) {
	q := r.base(schoolID).
		Select("ge.company_id AS company_id, c.name AS company_name, COALESCE(c.industry, '') AS industry, ge.graduation_year AS graduation_year, COUNT(*) AS count")
	if f.Year > 0 {
		q = q.Where("ge.graduation_year = ?", f.Year)
	}
	if f.Industry != "" {
		q = q.Where("c.industry = ?", f.Industry)
	}
	var out []GraduateOutcomeRow
	err := q.Group("ge.company_id, c.name, c.industry, ge.graduation_year").Scan(&out).Error
	return out, err
}

// FilterOptions は学校の実績にある年度と業界を返す（絞り込みの選択肢）。
func (r *GraduateOutcomeRepository) FilterOptions(schoolID uint) (years []int, industries []string, err error) {
	if err = r.base(schoolID).Distinct().Order("ge.graduation_year DESC").Pluck("ge.graduation_year", &years).Error; err != nil {
		return nil, nil, err
	}
	err = r.base(schoolID).Where("c.industry <> ''").Distinct().Order("c.industry").Pluck("c.industry", &industries).Error
	return years, industries, err
}

// GraduatePositionCount は職種ごとの人数。職種が未登録なら Title は空。
type GraduatePositionCount struct {
	Title string
	Count int
}

// CompanyCounts は1社について、学校の卒業生の年度別と職種別の人数を返す。
func (r *GraduateOutcomeRepository) CompanyCounts(schoolID, companyID uint) ([]GraduateOutcomeRow, []GraduatePositionCount, error) {
	var byYear []GraduateOutcomeRow
	if err := r.base(schoolID).Where("ge.company_id = ?", companyID).
		Select("ge.company_id AS company_id, c.name AS company_name, COALESCE(c.industry, '') AS industry, ge.graduation_year AS graduation_year, COUNT(*) AS count").
		Group("ge.company_id, c.name, c.industry, ge.graduation_year").
		Order("ge.graduation_year DESC").Scan(&byYear).Error; err != nil {
		return nil, nil, err
	}
	var byPosition []GraduatePositionCount
	err := r.base(schoolID).Where("ge.company_id = ?", companyID).
		Joins("LEFT JOIN company_job_positions jp ON jp.id = ge.job_position_id").
		Select("COALESCE(jp.title, '') AS title, COUNT(*) AS count").
		Group("jp.title").Order("count DESC").Scan(&byPosition).Error
	return byYear, byPosition, err
}
