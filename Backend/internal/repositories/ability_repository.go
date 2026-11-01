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

// ErrAbilityAlreadySubmitted は提出済みの受検へ回答・提出しようとしたとき。
var ErrAbilityAlreadySubmitted = errors.New("ability assessment already submitted")

// ErrAbilitySessionExists は同じ検査の受検中が既にあるとき（並行して開始された側）。
var ErrAbilitySessionExists = errors.New("ability session already in progress")

// AbilityRepository は能力検査（SOCAIAGENT-433）の永続化。
type AbilityRepository struct {
	db *gorm.DB
}

func NewAbilityRepository(db *gorm.DB) *AbilityRepository {
	return &AbilityRepository{db: db}
}

// ListTemplates は受検できる能力検査。運営標準のみなので絞り込みは無い。
func (r *AbilityRepository) ListTemplates() ([]models.AbilityTemplate, error) {
	var ts []models.AbilityTemplate
	err := r.db.Order("id ASC").Find(&ts).Error
	return ts, err
}

func (r *AbilityRepository) FindTemplate(id uint) (*models.AbilityTemplate, error) {
	var t models.AbilityTemplate
	if err := r.db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// ListQuestions は受検開始時に凍結した版の問題を返す。
//
// 正解（correct_choice）を含む行をそのまま返す。採点に必要なためであり、
// レスポンスへ出さないのは呼び出し側（ビュー型）の責務。
func (r *AbilityRepository) ListQuestions(templateID uint, version int) ([]models.AbilityQuestion, error) {
	var qs []models.AbilityQuestion
	err := r.db.Where("template_id = ? AND template_version = ?", templateID, version).
		Order("position ASC").Find(&qs).Error
	return qs, err
}

func (r *AbilityRepository) ListSessionsByUser(userID uint) ([]models.AbilitySession, error) {
	var ss []models.AbilitySession
	err := r.db.Where("user_id = ?", userID).Order("id DESC").Find(&ss).Error
	return ss, err
}

func (r *AbilityRepository) FindInProgressSession(userID, templateID uint) (*models.AbilitySession, error) {
	var s models.AbilitySession
	err := r.db.Where("user_id = ? AND template_id = ? AND status = ?", userID, templateID, models.AbilityStatusInProgress).
		Order("id DESC").First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *AbilityRepository) CreateSession(s *models.AbilitySession) error {
	err := r.db.Create(s).Error
	if mysqlErr := (*mysql.MySQLError)(nil); errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrAbilitySessionExists
	}
	return err
}

// FindSessionByPublicID は本人の受検だけを返す。他人の public_id は見つからない扱い。
func (r *AbilityRepository) FindSessionByPublicID(userID uint, publicID string) (*models.AbilitySession, error) {
	var s models.AbilitySession
	if err := r.db.Where("public_id = ? AND user_id = ?", publicID, userID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// UpsertAnswer は自動保存。同じ問への選び直しを1行に畳む（UNIQUE (session_id, question_id)）。
func (r *AbilityRepository) UpsertAnswer(sessionID, questionID uint, choice int) error {
	now := time.Now()
	a := models.AbilityAnswer{SessionID: sessionID, QuestionID: questionID, Choice: choice, CreatedAt: now, UpdatedAt: now}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_id"}, {Name: "question_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"choice", "updated_at"}),
	}).Create(&a).Error
}

func (r *AbilityRepository) ListAnswers(sessionID uint) ([]models.AbilityAnswer, error) {
	var as []models.AbilityAnswer
	err := r.db.Where("session_id = ?", sessionID).Find(&as).Error
	return as, err
}

// CountAnswersBySessions は一覧の「何問まで答えたか」用。
func (r *AbilityRepository) CountAnswersBySessions(sessionIDs []uint) (map[uint]int, error) {
	out := map[uint]int{}
	if len(sessionIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		SessionID uint
		N         int
	}
	err := r.db.Model(&models.AbilityAnswer{}).Select("session_id, COUNT(*) AS n").
		Where("session_id IN ?", sessionIDs).Group("session_id").Scan(&rows).Error
	for _, x := range rows {
		out[x.SessionID] = x.N
	}
	return out, err
}

// Submit は提出を1トランザクションで確定する。
//
// 状態の更新を条件付き UPDATE にして、同時に2回押されても片方だけが通る
// （もう片方は ErrAbilityAlreadySubmitted）。user_weight_scores へは書かない。
func (r *AbilityRepository) Submit(s *models.AbilitySession, scores map[string]any) error {
	payload, err := json.Marshal(scores)
	if err != nil {
		return err
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		res := tx.Model(&models.AbilitySession{}).
			Where("id = ? AND status = ?", s.ID, models.AbilityStatusInProgress).
			Updates(map[string]any{"status": models.AbilityStatusSubmitted, "submitted_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrAbilityAlreadySubmitted
		}
		if err := tx.Create(&models.AbilityResult{
			SessionID: s.ID, UserID: s.UserID, SectionScores: string(payload), CreatedAt: now,
		}).Error; err != nil {
			return err
		}
		s.Status = models.AbilityStatusSubmitted
		s.SubmittedAt = &now
		return nil
	})
}

func (r *AbilityRepository) FindResult(sessionID uint) (map[string]any, error) {
	var res models.AbilityResult
	if err := r.db.Where("session_id = ?", sessionID).First(&res).Error; err != nil {
		return nil, err
	}
	out := map[string]any{}
	return out, json.Unmarshal([]byte(res.SectionScores), &out)
}
