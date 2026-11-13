package teacher

import (
	"errors"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"

	"gorm.io/gorm"
)

// #2028: 教員と学生の相談。

type memConsultation struct {
	guidances map[uint]*models.TeacherStudentGuidance
	messages  []models.GuidanceMessage
}

func newMem() *memConsultation {
	return &memConsultation{guidances: map[uint]*models.TeacherStudentGuidance{
		1: {ID: 1, StudentUserID: 10, TeacherUserID: 20, Kind: models.GuidanceKindResume, Message: "履歴書を見直しましょう"},
	}}
}

func (m *memConsultation) FindGuidance(id uint) (*models.TeacherStudentGuidance, error) {
	if g, ok := m.guidances[id]; ok {
		return g, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memConsultation) CreateQuestion(studentUserID uint, body string) (*models.TeacherStudentGuidance, error) {
	id := uint(len(m.guidances) + 1)
	g := &models.TeacherStudentGuidance{ID: id, StudentUserID: studentUserID, Kind: models.GuidanceKindQuestion, Message: body}
	m.guidances[id] = g
	m.messages = append(m.messages, models.GuidanceMessage{ID: uint(len(m.messages) + 1), GuidanceID: id, AuthorUserID: studentUserID, AuthorRole: models.GuidanceAuthorStudent, Body: body})
	return g, nil
}
func (m *memConsultation) CreateMessage(msg *models.GuidanceMessage) error {
	msg.ID = uint(len(m.messages) + 1)
	m.messages = append(m.messages, *msg)
	return nil
}
func (m *memConsultation) ListMessages(guidanceID uint) ([]models.GuidanceMessage, error) {
	var out []models.GuidanceMessage
	for _, msg := range m.messages {
		if msg.GuidanceID == guidanceID {
			out = append(out, msg)
		}
	}
	return out, nil
}
func (m *memConsultation) MarkReadBy(guidanceID uint, readerRole string, at time.Time) error {
	for i := range m.messages {
		if m.messages[i].GuidanceID == guidanceID && m.messages[i].AuthorRole != readerRole && m.messages[i].ReadAt == nil {
			m.messages[i].ReadAt = &at
		}
	}
	return nil
}
func (m *memConsultation) UnreadForStudent(studentUserID uint) (map[uint]int, error) {
	out := map[uint]int{}
	for _, msg := range m.messages {
		if g := m.guidances[msg.GuidanceID]; g != nil && g.StudentUserID == studentUserID && msg.AuthorRole == models.GuidanceAuthorTeacher && msg.ReadAt == nil {
			out[msg.GuidanceID]++
		}
	}
	return out, nil
}
func (m *memConsultation) TeacherInbox(*uint, int) ([]repositories.ConsultationInboxRow, error) {
	return []repositories.ConsultationInboxRow{{GuidanceID: 1, StudentName: "山田", Unread: 2}}, nil
}

func TestConsultation_学生の返信と先生の既読(t *testing.T) {
	store := newMem()
	s := NewConsultationService(store)

	// 他人の案内には返信できない（見つからない扱い）
	if _, err := s.StudentReply(99, 1, "質問です"); !errors.Is(err, ErrConsultationNotFound) {
		t.Fatalf("他人の案内: err = %v", err)
	}
	if _, err := s.StudentReply(10, 1, "  "); !errors.Is(err, ErrConsultationEmpty) {
		t.Fatalf("空の返信: err = %v", err)
	}
	if _, err := s.StudentReply(10, 1, strings.Repeat("あ", 1001)); !errors.Is(err, ErrConsultationTooLong) {
		t.Fatalf("長すぎる返信: err = %v", err)
	}
	view, err := s.StudentReply(10, 1, "どこを直せばよいですか？")
	if err != nil || len(view.Messages) != 1 || view.Message != "履歴書を見直しましょう" {
		t.Fatalf("view = %+v err = %v", view, err)
	}

	g, _ := s.FindGuidance(1)
	if _, err := s.TeacherReply(20, g, "自己PRを具体的にしましょう"); err != nil {
		t.Fatalf("teacher reply: %v", err)
	}
	// 先生が返信したら、学生の発言は既読
	if store.messages[0].ReadAt == nil {
		t.Fatal("先生が返信したのに、学生の発言が未読のまま")
	}
	// 学生には先生の未読が1件
	if unread, _ := s.UnreadForStudent(10); unread[1] != 1 {
		t.Fatalf("学生の未読 = %v", unread)
	}
	if _, err := s.StudentThread(10, 1); err != nil {
		t.Fatalf("student thread: %v", err)
	}
	if unread, _ := s.UnreadForStudent(10); unread[1] != 0 {
		t.Fatalf("開いたのに未読が残る: %v", unread)
	}
}

func TestConsultation_学生からの質問(t *testing.T) {
	store := newMem()
	s := NewConsultationService(store)
	view, err := s.Ask(10, " 面接の練習はどれくらいすればよいですか ")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if view.Kind != models.GuidanceKindQuestion || view.Message != "面接の練習はどれくらいすればよいですか" || len(view.Messages) != 1 {
		t.Fatalf("view = %+v", view)
	}
	if store.guidances[view.GuidanceID].TeacherUserID != 0 {
		t.Fatal("学生の質問は担当の先生を決めない（teacher_user_id = 0）")
	}
}

func TestConsultation_Inbox(t *testing.T) {
	items, err := NewConsultationService(newMem()).Inbox(nil)
	if err != nil || len(items) != 1 || items[0].Unread != 2 {
		t.Fatalf("items = %+v err = %v", items, err)
	}
}
