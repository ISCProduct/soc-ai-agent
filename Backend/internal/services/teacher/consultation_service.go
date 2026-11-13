package teacher

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/repositories"
)

// 教員と学生の相談（案内への返信・学生からの質問、#2028）。
//
// 教員の案内は一方向だったので、学生が返信・質問できるようにする。案内（teacher_student_guidances）を
// 1つのスレッドとし、発言を guidance_messages に積む。学校をまたぐ操作は、呼び出し側（コントローラ）で
// 学生の学校と教員の担当校を突き合わせて 403 にする。

var (
	ErrConsultationNotFound  = errors.New("consultation not found")
	ErrConsultationEmpty     = errors.New("consultation message is empty")
	ErrConsultationTooLong   = errors.New("consultation message is too long")
	maxConsultationBodyRunes = 1000
)

// ConsultationStore は相談の永続化（repositories.GuidanceMessageRepository が満たす）。
type ConsultationStore interface {
	FindGuidance(id uint) (*models.TeacherStudentGuidance, error)
	CreateQuestion(studentUserID uint, body string) (*models.TeacherStudentGuidance, error)
	CreateMessage(m *models.GuidanceMessage) error
	ListMessages(guidanceID uint) ([]models.GuidanceMessage, error)
	MarkReadBy(guidanceID uint, readerRole string, at time.Time) error
	UnreadForStudent(studentUserID uint) (map[uint]int, error)
	TeacherInbox(schoolID *uint, limit int) ([]repositories.ConsultationInboxRow, error)
}

type ConsultationService struct {
	store ConsultationStore
	now   func() time.Time
}

func NewConsultationService(store ConsultationStore) *ConsultationService {
	return &ConsultationService{store: store, now: time.Now}
}

// ThreadView はスレッド1件。案内の本文（教員の最初の案内、または学生の質問）と、その後の発言。
type ThreadView struct {
	GuidanceID    uint                     `json:"guidance_id"`
	StudentUserID uint                     `json:"student_user_id"`
	Kind          string                   `json:"kind"`
	Message       string                   `json:"message"`
	CreatedAt     time.Time                `json:"created_at"`
	Messages      []models.GuidanceMessage `json:"messages"`
}

func validateBody(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", ErrConsultationEmpty
	}
	if utf8.RuneCountInString(body) > maxConsultationBodyRunes {
		return "", ErrConsultationTooLong
	}
	return body, nil
}

// Ask は学生から教員への質問のスレッドを作る。
func (s *ConsultationService) Ask(studentUserID uint, body string) (*ThreadView, error) {
	body, err := validateBody(body)
	if err != nil {
		return nil, err
	}
	g, err := s.store.CreateQuestion(studentUserID, body)
	if err != nil {
		return nil, err
	}
	return s.thread(g)
}

// guidanceForStudent は学生本人の案内だけを返す（他人の案内は見つからない扱い）。
func (s *ConsultationService) guidanceForStudent(studentUserID, guidanceID uint) (*models.TeacherStudentGuidance, error) {
	g, err := s.store.FindGuidance(guidanceID)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && g.StudentUserID != studentUserID) {
		return nil, ErrConsultationNotFound
	}
	return g, err
}

// FindGuidance は案内を返す（教員側の学校の確認に使う）。無ければ ErrConsultationNotFound。
func (s *ConsultationService) FindGuidance(guidanceID uint) (*models.TeacherStudentGuidance, error) {
	g, err := s.store.FindGuidance(guidanceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrConsultationNotFound
	}
	return g, err
}

func (s *ConsultationService) thread(g *models.TeacherStudentGuidance) (*ThreadView, error) {
	msgs, err := s.store.ListMessages(g.ID)
	if err != nil {
		return nil, err
	}
	if msgs == nil {
		msgs = []models.GuidanceMessage{}
	}
	return &ThreadView{GuidanceID: g.ID, StudentUserID: g.StudentUserID, Kind: g.Kind, Message: g.Message, CreatedAt: g.CreatedAt, Messages: msgs}, nil
}

// StudentThread は学生本人のスレッドを返し、教員の発言を既読にする。
func (s *ConsultationService) StudentThread(studentUserID, guidanceID uint) (*ThreadView, error) {
	g, err := s.guidanceForStudent(studentUserID, guidanceID)
	if err != nil {
		return nil, err
	}
	if err := s.store.MarkReadBy(g.ID, models.GuidanceAuthorStudent, s.now()); err != nil {
		return nil, err
	}
	return s.thread(g)
}

// StudentReply は学生本人のスレッドに返信する。
func (s *ConsultationService) StudentReply(studentUserID, guidanceID uint, body string) (*ThreadView, error) {
	body, err := validateBody(body)
	if err != nil {
		return nil, err
	}
	g, err := s.guidanceForStudent(studentUserID, guidanceID)
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateMessage(&models.GuidanceMessage{GuidanceID: g.ID, AuthorUserID: studentUserID, AuthorRole: models.GuidanceAuthorStudent, Body: body}); err != nil {
		return nil, err
	}
	return s.thread(g)
}

// TeacherThread はスレッドを返し、学生の発言を既読にする（担当校の確認は呼び出し側で済ませる）。
func (s *ConsultationService) TeacherThread(g *models.TeacherStudentGuidance) (*ThreadView, error) {
	if err := s.store.MarkReadBy(g.ID, models.GuidanceAuthorTeacher, s.now()); err != nil {
		return nil, err
	}
	return s.thread(g)
}

// TeacherReply は教員がスレッドに返信する（担当校の確認は呼び出し側で済ませる）。
func (s *ConsultationService) TeacherReply(teacherUserID uint, g *models.TeacherStudentGuidance, body string) (*ThreadView, error) {
	body, err := validateBody(body)
	if err != nil {
		return nil, err
	}
	if err := s.store.CreateMessage(&models.GuidanceMessage{GuidanceID: g.ID, AuthorUserID: teacherUserID, AuthorRole: models.GuidanceAuthorTeacher, Body: body}); err != nil {
		return nil, err
	}
	// 返信したら、学生の発言は読んだことになる
	if err := s.store.MarkReadBy(g.ID, models.GuidanceAuthorTeacher, s.now()); err != nil {
		return nil, err
	}
	return s.thread(g)
}

// UnreadForStudent は学生の各スレッドの、教員の未読の発言の数を返す。
func (s *ConsultationService) UnreadForStudent(studentUserID uint) (map[uint]int, error) {
	return s.store.UnreadForStudent(studentUserID)
}

// InboxItem は教員の受信一覧の1行。
type InboxItem struct {
	GuidanceID    uint      `json:"guidance_id"`
	StudentUserID uint      `json:"student_user_id"`
	StudentName   string    `json:"student_name"`
	Kind          string    `json:"kind"`
	FirstMessage  string    `json:"first_message"`
	Unread        int       `json:"unread"`
	LastMessageAt time.Time `json:"last_message_at"`
}

// Inbox は学生の発言があるスレッドを、新しい順に返す。schoolID が非nilならその学校の学生だけ。
func (s *ConsultationService) Inbox(schoolID *uint) ([]InboxItem, error) {
	rows, err := s.store.TeacherInbox(schoolID, 100)
	if err != nil {
		return nil, err
	}
	out := make([]InboxItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, InboxItem(r))
	}
	return out, nil
}

// FormatConsultationError は入力の誤りを、利用者に見せる文言にする。
func FormatConsultationError(err error) string {
	switch {
	case errors.Is(err, ErrConsultationEmpty):
		return "メッセージを入力してください"
	case errors.Is(err, ErrConsultationTooLong):
		return "メッセージは1000文字以内にしてください"
	}
	return "送信できませんでした"
}
