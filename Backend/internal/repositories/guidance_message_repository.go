package repositories

import (
	"errors"
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
)

// GuidanceMessageRepository は案内のスレッド（教員と学生の相談、#2028）の永続化。
type GuidanceMessageRepository struct {
	db *gorm.DB
}

func NewGuidanceMessageRepository(db *gorm.DB) *GuidanceMessageRepository {
	return &GuidanceMessageRepository{db: db}
}

// FindGuidance は案内を1件返す。無ければ repository.ErrNotFound（= gorm.ErrRecordNotFound）。
func (r *GuidanceMessageRepository) FindGuidance(id uint) (*models.TeacherStudentGuidance, error) {
	var g models.TeacherStudentGuidance
	if err := r.db.First(&g, id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// ErrOpenQuestionLimit は、未回答の質問が上限に達していて新しい質問を作らなかったこと。
var ErrOpenQuestionLimit = errors.New("open question limit reached")

// CreateQuestion は学生からの質問のスレッド（種類 question の案内）と最初の発言を、1つのトランザクションで作る。
//
// 未回答の質問が maxOpen 件以上あれば ErrOpenQuestionLimit を返して作らない。数えてから作るまでを
// 1つのトランザクションにし、学生の users 行をロックする（同時に送られても上限を超えないように）。
func (r *GuidanceMessageRepository) CreateQuestion(studentUserID uint, body string, maxOpen int64) (*models.TeacherStudentGuidance, error) {
	g := &models.TeacherStudentGuidance{StudentUserID: studentUserID, Kind: models.GuidanceKindQuestion, Message: body}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var locked uint
		if err := tx.Raw("SELECT id FROM users WHERE id = ? FOR UPDATE", studentUserID).Scan(&locked).Error; err != nil {
			return err
		}
		open, err := countOpenQuestions(tx, studentUserID)
		if err != nil {
			return err
		}
		if open >= maxOpen {
			return ErrOpenQuestionLimit
		}
		if err := tx.Create(g).Error; err != nil {
			return err
		}
		return tx.Create(&models.GuidanceMessage{
			GuidanceID: g.ID, AuthorUserID: studentUserID, AuthorRole: models.GuidanceAuthorStudent, Body: body,
		}).Error
	})
	return g, err
}

// CreateMessage は発言を1件足す。学生が返したスレッドは、閉じていても開き直す（教員の返答を見落とさないように）。
func (r *GuidanceMessageRepository) CreateMessage(m *models.GuidanceMessage) error {
	return r.CreateMessageAndMarkRead(m, 0, time.Time{})
}

// CreateMessageAndMarkRead は発言を足し、同じトランザクションで、書き手の側が相手の発言を
// seenUpToID まで読んだことにする（0 なら既読にしない）。返信の保存と既読を分けると、
// 既読だけ失敗して 500 になり、再送で返信が二重になるため。
func (r *GuidanceMessageRepository) CreateMessageAndMarkRead(m *models.GuidanceMessage, seenUpToID uint, at time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.TeacherStudentGuidance{}).Where("id = ?", m.GuidanceID).
			Updates(map[string]any{"dismissed_at": nil, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		if seenUpToID == 0 {
			return nil
		}
		return markRead(tx, m.GuidanceID, m.AuthorRole, seenUpToID, at)
	})
}

// ListMessages はスレッドの発言を古い順に返す。
func (r *GuidanceMessageRepository) ListMessages(guidanceID uint) ([]models.GuidanceMessage, error) {
	var out []models.GuidanceMessage
	err := r.db.Where("guidance_id = ?", guidanceID).Order("id ASC").Find(&out).Error
	return out, err
}

// MarkReadBy は readerRole の側が、相手側の発言を upToID まで読んだことにする（未読の発言に read_at を付ける）。
// 画面に出した最後の発言の ID までに限るのは、表示のあとに届いた発言まで既読にしないため。
func (r *GuidanceMessageRepository) MarkReadBy(guidanceID uint, readerRole string, upToID uint, at time.Time) error {
	return markRead(r.db, guidanceID, readerRole, upToID, at)
}

func markRead(db *gorm.DB, guidanceID uint, readerRole string, upToID uint, at time.Time) error {
	return db.Model(&models.GuidanceMessage{}).
		Where("guidance_id = ? AND author_role <> ? AND read_at IS NULL AND id <= ?", guidanceID, readerRole, upToID).
		Update("read_at", at).Error
}

// CountOpenQuestions は学生の質問のうち、教員がまだ1度も答えていないものの数を返す。
func (r *GuidanceMessageRepository) CountOpenQuestions(studentUserID uint) (int64, error) {
	return countOpenQuestions(r.db, studentUserID)
}

func countOpenQuestions(db *gorm.DB, studentUserID uint) (int64, error) {
	var n int64
	err := db.Table("teacher_student_guidances g").
		Where("g.student_user_id = ? AND g.kind = ?", studentUserID, models.GuidanceKindQuestion).
		Where("NOT EXISTS (SELECT 1 FROM guidance_messages m WHERE m.guidance_id = g.id AND m.author_role = ?)", models.GuidanceAuthorTeacher).
		Count(&n).Error
	return n, err
}

// StudentThreadCursor は学生の相談一覧の続きの位置（最後に返した行の更新日時と ID）。
type StudentThreadCursor struct {
	UpdatedAt time.Time
	ID        uint
}

// StudentThreadRow は学生の相談一覧の1行。閉じた（dismissed）案内も含む。
type StudentThreadRow struct {
	GuidanceID uint
	Kind       string
	Message    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Unread     int
}

// ListStudentThreads は学生本人の案内と質問を、新しく動いた順（updated_at, id の降順）に返す。
// before が非nilなら、その位置より後ろ（古い側）だけ。
func (r *GuidanceMessageRepository) ListStudentThreads(studentUserID uint, before *StudentThreadCursor, limit int) ([]StudentThreadRow, error) {
	q := r.db.Table("teacher_student_guidances g").
		Select(`g.id AS guidance_id, g.kind AS kind, g.message AS message, g.created_at AS created_at, g.updated_at AS updated_at,
			(SELECT COUNT(*) FROM guidance_messages m WHERE m.guidance_id = g.id AND m.author_role = ? AND m.read_at IS NULL) AS unread`,
			models.GuidanceAuthorTeacher).
		Where("g.student_user_id = ?", studentUserID)
	if before != nil {
		q = q.Where("(g.updated_at < ? OR (g.updated_at = ? AND g.id < ?))", before.UpdatedAt, before.UpdatedAt, before.ID)
	}
	var out []StudentThreadRow
	err := q.Order("g.updated_at DESC, g.id DESC").Limit(limit).Scan(&out).Error
	return out, err
}

// UnreadForStudent は学生の各スレッドで、教員の未読の発言の数を返す。
func (r *GuidanceMessageRepository) UnreadForStudent(studentUserID uint) (map[uint]int, error) {
	var rows []struct {
		GuidanceID uint
		Count      int
	}
	err := r.db.Table("guidance_messages m").
		Select("m.guidance_id AS guidance_id, COUNT(*) AS count").
		Joins("JOIN teacher_student_guidances g ON g.id = m.guidance_id").
		Where("g.student_user_id = ? AND m.author_role = ? AND m.read_at IS NULL", studentUserID, models.GuidanceAuthorTeacher).
		Group("m.guidance_id").Scan(&rows).Error
	out := make(map[uint]int, len(rows))
	for _, r := range rows {
		out[r.GuidanceID] = r.Count
	}
	return out, err
}

// ConsultationInboxRow は教員の受信一覧の1行。
type ConsultationInboxRow struct {
	GuidanceID    uint
	StudentUserID uint
	StudentName   string
	Kind          string
	FirstMessage  string
	Unread        int
	LastMessageAt time.Time
	LastMessageID uint
}

// TeacherInbox は学生の発言があるスレッドを、新しい発言の順（最後の発言の ID の降順）に返す。
// schoolID が非nilならその学校（同じ組織）の学生に限る。beforeID が 0 でなければ、最後の発言の ID がそれより小さいものだけ（続きの取得）。
// 学生は ResolveStudentSchool と同じ条件（学生ロール・管理者でない・ゲストでない・退会していない）に限る。
func (r *GuidanceMessageRepository) TeacherInbox(schoolID *uint, beforeID uint, limit int) ([]ConsultationInboxRow, error) {
	q := r.db.Table("teacher_student_guidances g").
		Select(`g.id AS guidance_id, g.student_user_id AS student_user_id, u.name AS student_name, g.kind AS kind, g.message AS first_message,
			SUM(CASE WHEN m.author_role = ? AND m.read_at IS NULL THEN 1 ELSE 0 END) AS unread,
			MAX(m.created_at) AS last_message_at, MAX(m.id) AS last_message_id`, models.GuidanceAuthorStudent).
		Joins("JOIN guidance_messages m ON m.guidance_id = g.id").
		Joins("JOIN users u ON u.id = g.student_user_id").
		Where("u.withdrawn_at IS NULL AND u.role = ? AND u.is_admin = ? AND u.is_guest = ?", models.UserRoleStudent, false, false).
		Where("EXISTS (SELECT 1 FROM guidance_messages sm WHERE sm.guidance_id = g.id AND sm.author_role = ?)", models.GuidanceAuthorStudent)
	if schoolID != nil {
		q = q.Where("u.school_id = ?", *schoolID).
			Where("u.organization_id = (SELECT organization_id FROM schools WHERE id = ?)", *schoolID)
	}
	q = q.Group("g.id, g.student_user_id, u.name, g.kind, g.message")
	if beforeID > 0 {
		q = q.Having("MAX(m.id) < ?", beforeID)
	}
	var out []ConsultationInboxRow
	err := q.Order("last_message_id DESC").Limit(limit).Scan(&out).Error
	return out, err
}
