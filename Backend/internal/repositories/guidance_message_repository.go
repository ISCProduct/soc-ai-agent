package repositories

import (
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

// FindGuidance は案内を1件返す。無ければ gorm.ErrRecordNotFound。
func (r *GuidanceMessageRepository) FindGuidance(id uint) (*models.TeacherStudentGuidance, error) {
	var g models.TeacherStudentGuidance
	if err := r.db.First(&g, id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// CreateQuestion は学生からの質問のスレッド（種類 question の案内）と最初の発言を、1つのトランザクションで作る。
func (r *GuidanceMessageRepository) CreateQuestion(studentUserID uint, body string) (*models.TeacherStudentGuidance, error) {
	g := &models.TeacherStudentGuidance{StudentUserID: studentUserID, Kind: models.GuidanceKindQuestion, Message: body}
	err := r.db.Transaction(func(tx *gorm.DB) error {
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
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		return tx.Model(&models.TeacherStudentGuidance{}).Where("id = ?", m.GuidanceID).
			Updates(map[string]any{"dismissed_at": nil, "updated_at": time.Now()}).Error
	})
}

// ListMessages はスレッドの発言を古い順に返す。
func (r *GuidanceMessageRepository) ListMessages(guidanceID uint) ([]models.GuidanceMessage, error) {
	var out []models.GuidanceMessage
	err := r.db.Where("guidance_id = ?", guidanceID).Order("id ASC").Find(&out).Error
	return out, err
}

// MarkReadBy は readerRole の側が読んだことにする（相手側の未読の発言に read_at を付ける）。
func (r *GuidanceMessageRepository) MarkReadBy(guidanceID uint, readerRole string, at time.Time) error {
	return r.db.Model(&models.GuidanceMessage{}).
		Where("guidance_id = ? AND author_role <> ? AND read_at IS NULL", guidanceID, readerRole).
		Update("read_at", at).Error
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
}

// TeacherInbox は学生の発言があるスレッドを、新しい発言の順に返す。schoolID が非nilならその学校（同じ組織）の学生に限る。
func (r *GuidanceMessageRepository) TeacherInbox(schoolID *uint, limit int) ([]ConsultationInboxRow, error) {
	q := r.db.Table("teacher_student_guidances g").
		Select(`g.id AS guidance_id, g.student_user_id AS student_user_id, u.name AS student_name, g.kind AS kind, g.message AS first_message,
			SUM(CASE WHEN m.author_role = ? AND m.read_at IS NULL THEN 1 ELSE 0 END) AS unread,
			MAX(m.created_at) AS last_message_at`, models.GuidanceAuthorStudent).
		Joins("JOIN guidance_messages m ON m.guidance_id = g.id").
		Joins("JOIN users u ON u.id = g.student_user_id").
		Where("u.withdrawn_at IS NULL").
		Where("EXISTS (SELECT 1 FROM guidance_messages sm WHERE sm.guidance_id = g.id AND sm.author_role = ?)", models.GuidanceAuthorStudent)
	if schoolID != nil {
		q = q.Where("u.school_id = ?", *schoolID).
			Where("u.organization_id = (SELECT organization_id FROM schools WHERE id = ?)", *schoolID)
	}
	var out []ConsultationInboxRow
	err := q.Group("g.id, g.student_user_id, u.name, g.kind, g.message").
		Order("last_message_at DESC").Limit(limit).Scan(&out).Error
	return out, err
}
