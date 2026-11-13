package repositories

import (
	"time"

	"gorm.io/gorm"
)

// PortfolioRepository は自己分析ポートフォリオ（#2030）の材料を、本人の分だけ引く。
// どのクエリも user_id で絞る。教員向けの詳細（interview_reports.teacher_report_json）は読まない。
type PortfolioRepository struct {
	db *gorm.DB
}

func NewPortfolioRepository(db *gorm.DB) *PortfolioRepository {
	return &PortfolioRepository{db: db}
}

// PortfolioInterview は面接レポート1件（学生向けの項目だけ）。
type PortfolioInterview struct {
	SessionID        uint      `json:"session_id"`
	CreatedAt        time.Time `json:"created_at"`
	SummaryText      string    `json:"summary_text"`
	ScoresJSON       string    `json:"-"`
	StrengthsJSON    string    `json:"-"`
	ImprovementsJSON string    `json:"-"`
}

// RecentInterviews は終了済みでレポートのある面接を、新しい順に limit 件返す。削除した面接は除く。
func (r *PortfolioRepository) RecentInterviews(userID uint, limit int) ([]PortfolioInterview, error) {
	var out []PortfolioInterview
	err := r.db.Table("interview_sessions s").
		Select(`s.id AS session_id, s.created_at,
			COALESCE(ir.summary_text, '') AS summary_text,
			COALESCE(ir.scores_json, '') AS scores_json,
			COALESCE(ir.strengths_json, '') AS strengths_json,
			COALESCE(ir.improvements_json, '') AS improvements_json`).
		Joins("JOIN interview_reports ir ON ir.session_id = s.id").
		Where("s.user_id = ? AND s.status = ? AND s.deleted_at IS NULL", userID, "finished").
		Order("s.created_at DESC, s.id DESC").Limit(limit).
		Scan(&out).Error
	return out, err
}

// PortfolioES は ES 添削1件。本文（es_text）は長く、提出用の要約には向かないので含めない。
type PortfolioES struct {
	ID                 uint      `json:"id"`
	CreatedAt          time.Time `json:"created_at"`
	Question           string    `json:"question"`
	CompanyName        *string   `json:"company_name,omitempty"`
	SpecificityScore   uint8     `json:"specificity_score"`
	StarScore          uint8     `json:"star_score"`
	LengthBalanceScore uint8     `json:"length_balance_score"`
	CompanyFitScore    *uint8    `json:"company_fit_score,omitempty"`
	Feedback           string    `json:"feedback"`
}

// RecentESReviews は ES 添削を新しい順に limit 件返す。
func (r *PortfolioRepository) RecentESReviews(userID uint, limit int) ([]PortfolioES, error) {
	var out []PortfolioES
	err := r.db.Table("es_reviews").
		Select("id, created_at, question, company_name, specificity_score, star_score, length_balance_score, company_fit_score, feedback").
		Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").Limit(limit).
		Scan(&out).Error
	return out, err
}

// PortfolioAssessment は適性検査（学校の診断）の結果1件。
type PortfolioAssessment struct {
	ResultID           uint      `json:"result_id"`
	CreatedAt          time.Time `json:"created_at"`
	Title              string    `json:"title"`
	CategoryScoresJSON string    `json:"-"`
}

// RecentAssessments は適性検査の結果を新しい順に limit 件返す。
func (r *PortfolioRepository) RecentAssessments(userID uint, limit int) ([]PortfolioAssessment, error) {
	var out []PortfolioAssessment
	err := r.db.Table("assessment_results ar").
		Select("ar.id AS result_id, ar.created_at, COALESCE(t.title, '') AS title, ar.category_scores AS category_scores_json").
		Joins("LEFT JOIN assessment_sessions s ON s.id = ar.session_id").
		Joins("LEFT JOIN assessment_templates t ON t.id = s.template_id").
		Where("ar.user_id = ?", userID).
		Order("ar.created_at DESC, ar.id DESC").Limit(limit).
		Scan(&out).Error
	return out, err
}
