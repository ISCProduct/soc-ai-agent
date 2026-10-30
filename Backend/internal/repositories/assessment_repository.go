package repositories

import (
	"encoding/json"
	"errors"
	"time"

	"Backend/internal/models"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrAssessmentAlreadySubmitted は提出済みの受検へ回答・提出しようとしたとき（#1678）。
var ErrAssessmentAlreadySubmitted = errors.New("assessment already submitted")

// ErrAssessmentSessionExists は同じ検査の受検中が既にあるとき（並行して開始された側）。
var ErrAssessmentSessionExists = errors.New("assessment session already in progress")

// AssessmentRepository は適性検査（#1678）の永続化。
type AssessmentRepository struct {
	db *gorm.DB
}

func NewAssessmentRepository(db *gorm.DB) *AssessmentRepository {
	return &AssessmentRepository{db: db}
}

// ListTemplates は学生に見せる検査。運営標準に加え、本人の学校の独自テンプレートを返す（#1675）。
// 他校のテンプレートは返さない（schoolID が nil の学生は運営標準だけ）。
func (r *AssessmentRepository) ListTemplates(schoolID *uint) ([]models.AssessmentTemplate, error) {
	var ts []models.AssessmentTemplate
	q := r.db.Order("id ASC")
	if schoolID == nil {
		q = q.Where("school_id IS NULL")
	} else {
		q = q.Where("school_id IS NULL OR school_id = ?", *schoolID)
	}
	err := q.Find(&ts).Error
	return ts, err
}

// FindTemplate は id でテンプレートを引く。可視範囲の検査はしない。
// 本人の受検（自分の session が指すテンプレート）を読むときに使う。受検を始めるときは FindVisibleTemplate。
func (r *AssessmentRepository) FindTemplate(id uint) (*models.AssessmentTemplate, error) {
	var t models.AssessmentTemplate
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// FindVisibleTemplate は学生が受検を始められるテンプレートだけを返す。
// 他校の独自テンプレートは gorm.ErrRecordNotFound（存在を教えない）。
func (r *AssessmentRepository) FindVisibleTemplate(id uint, schoolID *uint) (*models.AssessmentTemplate, error) {
	var t models.AssessmentTemplate
	q := r.db.Where("id = ?", id)
	if schoolID == nil {
		q = q.Where("school_id IS NULL")
	} else {
		q = q.Where("(school_id IS NULL OR school_id = ?)", *schoolID)
	}
	if err := q.First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// ListQuestions は受検開始時に凍結した版の問題を返す。
func (r *AssessmentRepository) ListQuestions(templateID uint, version int) ([]models.AssessmentQuestion, error) {
	var qs []models.AssessmentQuestion
	err := r.db.Where("template_id = ? AND template_version = ?", templateID, version).
		Order("position ASC").Find(&qs).Error
	return qs, err
}

func (r *AssessmentRepository) ListSessionsByUser(userID uint) ([]models.AssessmentSession, error) {
	var ss []models.AssessmentSession
	err := r.db.Where("user_id = ?", userID).Order("id DESC").Find(&ss).Error
	return ss, err
}

func (r *AssessmentRepository) FindInProgressSession(userID, templateID uint) (*models.AssessmentSession, error) {
	var s models.AssessmentSession
	err := r.db.Where("user_id = ? AND template_id = ? AND status = ?", userID, templateID, models.AssessmentStatusInProgress).
		Order("id DESC").First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *AssessmentRepository) CreateSession(s *models.AssessmentSession) error {
	err := r.db.Create(s).Error
	if mysqlErr := (*mysql.MySQLError)(nil); errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrAssessmentSessionExists
	}
	return err
}

// FindSessionByPublicID は本人の受検だけを返す。他人の public_id は見つからない扱い。
func (r *AssessmentRepository) FindSessionByPublicID(userID uint, publicID string) (*models.AssessmentSession, error) {
	var s models.AssessmentSession
	if err := r.db.Where("public_id = ? AND user_id = ?", publicID, userID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// UpsertAnswer は自動保存。同じ回答の再送や選び直しを1行に畳む（UNIQUE (session_id, question_id)）。
func (r *AssessmentRepository) UpsertAnswer(sessionID, questionID uint, value int) error {
	now := time.Now()
	a := models.AssessmentAnswer{SessionID: sessionID, QuestionID: questionID, Value: value, CreatedAt: now, UpdatedAt: now}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_id"}, {Name: "question_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&a).Error
}

func (r *AssessmentRepository) ListAnswers(sessionID uint) ([]models.AssessmentAnswer, error) {
	var as []models.AssessmentAnswer
	err := r.db.Where("session_id = ?", sessionID).Find(&as).Error
	return as, err
}

// CountAnswersBySessions は一覧の「何問まで答えたか」用。
func (r *AssessmentRepository) CountAnswersBySessions(sessionIDs []uint) (map[uint]int, error) {
	out := map[uint]int{}
	if len(sessionIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		SessionID uint
		N         int
	}
	err := r.db.Model(&models.AssessmentAnswer{}).Select("session_id, COUNT(*) AS n").
		Where("session_id IN ?", sessionIDs).Group("session_id").Scan(&rows).Error
	for _, x := range rows {
		out[x.SessionID] = x.N
	}
	return out, err
}

// Submit は提出を1トランザクションで確定する。
//
// 状態の更新を条件付き UPDATE にして、同時に2回押されても片方だけが通る（もう片方は
// ErrAssessmentAlreadySubmitted）。スコアは UserWeightScoreRepository を通して
// source=assessment で書く。正典外カテゴリはそこで弾かれ、全体がロールバックされる。
//
// writeScores が false のとき user_weight_scores へは書かない。学校独自テンプレートはマッチングに
// 使わないため（#1675）。assessment_results には常に残す。
func (r *AssessmentRepository) Submit(s *models.AssessmentSession, scores map[string]float64, scoreSessionID string, writeScores bool) error {
	payload, err := json.Marshal(scores)
	if err != nil {
		return err
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		res := tx.Model(&models.AssessmentSession{}).
			Where("id = ? AND status = ?", s.ID, models.AssessmentStatusInProgress).
			Updates(map[string]any{"status": models.AssessmentStatusSubmitted, "submitted_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrAssessmentAlreadySubmitted
		}
		if err := tx.Create(&models.AssessmentResult{SessionID: s.ID, UserID: s.UserID, CategoryScores: string(payload), CreatedAt: now}).Error; err != nil {
			return err
		}
		if !writeScores {
			s.Status = models.AssessmentStatusSubmitted
			s.SubmittedAt = &now
			return nil
		}
		scoreRepo := NewUserWeightScoreRepository(tx)
		for category, score := range scores {
			if err := scoreRepo.SetScoreWithSource(s.UserID, scoreSessionID, category, int(score+0.5), models.ScoreSourceAssessment); err != nil {
				return err
			}
		}
		s.Status = models.AssessmentStatusSubmitted
		s.SubmittedAt = &now
		return nil
	})
}

func (r *AssessmentRepository) FindResult(sessionID uint) (map[string]float64, error) {
	var res models.AssessmentResult
	if err := r.db.Where("session_id = ?", sessionID).First(&res).Error; err != nil {
		return nil, err
	}
	out := map[string]float64{}
	return out, json.Unmarshal([]byte(res.CategoryScores), &out)
}

// LatestStandardScores はマッチングへ合流させる、最新の標準検査の結果（#1678）。
// 受検していなければ nil, nil。
func (r *AssessmentRepository) LatestStandardScores(userID uint) (map[string]float64, error) {
	var res models.AssessmentResult
	err := r.db.Table("assessment_results AS r").Select("r.*").
		Joins("JOIN assessment_sessions s ON s.id = r.session_id").
		Joins("JOIN assessment_templates t ON t.id = s.template_id").
		Where("r.user_id = ? AND t.is_standard = ?", userID, true).
		Order("r.created_at DESC, r.id DESC").
		Take(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	return out, json.Unmarshal([]byte(res.CategoryScores), &out)
}

// FindUserSchoolID は学生の所属校。未設定なら nil（運営標準だけ見える）。
func (r *AssessmentRepository) FindUserSchoolID(userID uint) (*uint, error) {
	var u struct{ SchoolID *uint }
	if err := r.db.Table("users").Select("school_id").Where("id = ?", userID).Take(&u).Error; err != nil {
		return nil, err
	}
	return u.SchoolID, nil
}

// FindSchoolOrganizationID は学校の所属学園。学校独自テンプレートのテナント境界に使う。
func (r *AssessmentRepository) FindSchoolOrganizationID(schoolID uint) (uint, error) {
	var sc models.School
	if err := r.db.Select("id", "organization_id").Where("id = ?", schoolID).Take(&sc).Error; err != nil {
		return 0, err
	}
	return sc.OrganizationID, nil
}

// CreateSchoolTemplate は学校独自テンプレートと問題を1トランザクションで作る。
// is_standard は常に false（マッチングに合流させない）。
func (r *AssessmentRepository) CreateSchoolTemplate(t *models.AssessmentTemplate, qs []models.AssessmentQuestion) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		t.IsStandard = false
		if err := tx.Create(t).Error; err != nil {
			return err
		}
		for i := range qs {
			qs[i].TemplateID = t.ID
			qs[i].TemplateVersion = t.Version
		}
		return tx.Create(&qs).Error
	})
}

// ListSchoolTemplates は学校独自テンプレートだけ（運営標準は含まない）。新しい順。
func (r *AssessmentRepository) ListSchoolTemplates(schoolID uint) ([]models.AssessmentTemplate, error) {
	var ts []models.AssessmentTemplate
	err := r.db.Where("school_id = ?", schoolID).Order("id DESC").Find(&ts).Error
	return ts, err
}

// FindSchoolTemplate は指定校の独自テンプレート。他校のものは gorm.ErrRecordNotFound。
func (r *AssessmentRepository) FindSchoolTemplate(id, schoolID uint) (*models.AssessmentTemplate, error) {
	var t models.AssessmentTemplate
	if err := r.db.Where("id = ? AND school_id = ?", id, schoolID).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// SchoolStudent は先生向け一覧に出す学生の最小限の情報。メール等は持たない。
type SchoolStudent struct {
	ID   uint
	Name string
}

// ListSchoolStudents は指定校・指定学園の在籍学生（退会済み・職員を除く）。
// organization_id も条件に入れ、学校IDの付け違いが他学園の学生を見せないようにする。
func (r *AssessmentRepository) ListSchoolStudents(schoolID, organizationID uint) ([]SchoolStudent, error) {
	var out []SchoolStudent
	err := r.db.Table("users").Select("id, name").
		Where("school_id = ? AND organization_id = ? AND role = ? AND withdrawn_at IS NULL",
			schoolID, organizationID, models.UserRoleStudent).
		Order("id ASC").Scan(&out).Error
	return out, err
}

// ListSessionsByTemplate はテンプレートの全受検（ユーザーごとの最新を選ぶのは呼び出し側）。id 降順。
func (r *AssessmentRepository) ListSessionsByTemplate(templateID uint) ([]models.AssessmentSession, error) {
	var ss []models.AssessmentSession
	err := r.db.Where("template_id = ?", templateID).Order("id DESC").Find(&ss).Error
	return ss, err
}

// ListResultsBySessions は session_id -> カテゴリ別スコア。
func (r *AssessmentRepository) ListResultsBySessions(sessionIDs []uint) (map[uint]map[string]float64, error) {
	out := map[uint]map[string]float64{}
	if len(sessionIDs) == 0 {
		return out, nil
	}
	var rows []models.AssessmentResult
	if err := r.db.Where("session_id IN ?", sessionIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		m := map[string]float64{}
		if err := json.Unmarshal([]byte(row.CategoryScores), &m); err != nil {
			return nil, err
		}
		out[row.SessionID] = m
	}
	return out, nil
}
