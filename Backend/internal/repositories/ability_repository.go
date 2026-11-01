package repositories

import (
	"encoding/json"
	"errors"
	"time"

	"Backend/internal/models"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
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

// ErrAbilityAnswerRejected は条件付き書き込みが通らなかったとき（提出済み、または締め切り後）。
// どちらだったかは呼び出し側がセッションを読み直して決める。
var ErrAbilityAnswerRejected = errors.New("ability answer rejected")

// UpsertAnswer は自動保存。同じ問への選び直しを1行に畳む（UNIQUE (session_id, question_id)）。
//
// 「受検中で、まだ締め切り前」を SQL の条件に入れて1文で書く。
// サービス側で状態と締め切りを確かめてから書く形だと、確認と書き込みの間に
// 提出が確定したり締め切りを過ぎたりしたとき、採点に入らない回答が保存される。
// 画面は保存済みの回答を出すので、結果と表示が食い違う。
// 条件に合わなければ 0 行で返り、ErrAbilityAnswerRejected にする。
func (r *AbilityRepository) UpsertAnswer(sessionID, questionID uint, choice int) error {
	now := time.Now()
	res := r.db.Exec(`
		INSERT INTO `+"`ability_answers`"+` (`+"`session_id`"+`, `+"`question_id`"+`, `+"`choice`"+`, `+"`created_at`"+`, `+"`updated_at`"+`)
		SELECT ?, ?, ?, ?, ? FROM `+"`ability_sessions`"+` s
		WHERE s.id = ? AND s.status = ? AND (s.deadline_at IS NULL OR s.deadline_at >= ?)
		ON DUPLICATE KEY UPDATE `+"`choice`"+` = VALUES(`+"`choice`"+`), `+"`updated_at`"+` = VALUES(`+"`updated_at`"+`)`,
		sessionID, questionID, choice, now, now,
		sessionID, models.AbilityStatusInProgress, now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrAbilityAnswerRejected
	}
	return nil
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

// Submit は提出を1トランザクションで確定する。採点は score に渡して同じ中で行う。
//
// 手順をこの順にするのが肝。
//  1. 条件付き UPDATE で受検中→提出済みにする（同時に2回押されても片方だけ通る）
//  2. 同じトランザクションで回答を読む
//  3. score で採点して結果を書く
//
// 1 を先に済ませると、以降に走る UpsertAnswer は条件（status = in_progress）に
// 合わず弾かれる。1 より前に確定していた回答は 2 の読み取りに入る。
// 採点した集合と保存されている回答が必ず一致する。
// 採点を呼び出し側で先に済ませる形だと、その後に確定した回答が保存だけされて
// 採点に入らない状態ができる。
//
// user_weight_scores へは書かない（能力検査の正答率は10カテゴリへ変換できない）。
func (r *AbilityRepository) Submit(s *models.AbilitySession, score func(answers map[uint]int) (map[string]any, error)) error {
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

		var rows []models.AbilityAnswer
		if err := tx.Where("session_id = ?", s.ID).Find(&rows).Error; err != nil {
			return err
		}
		answers := make(map[uint]int, len(rows))
		for _, a := range rows {
			answers[a.QuestionID] = a.Choice
		}
		scores, err := score(answers)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(scores)
		if err != nil {
			return err
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
