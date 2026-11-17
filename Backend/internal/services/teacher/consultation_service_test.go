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
	// afterList は ListMessages の直後に呼ぶ（取得と既読の間に発言が届く競合を再現する）
	afterList func()
	listErr   error
	inbox     []repositories.ConsultationInboxRow
	gotLimit  int
	gotBefore uint
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
	return m.CreateMessageAndMarkRead(msg, 0, time.Time{})
}
func (m *memConsultation) CreateMessageAndMarkRead(msg *models.GuidanceMessage, seenUpToID uint, at time.Time) error {
	msg.ID = uint(len(m.messages) + 1)
	m.messages = append(m.messages, *msg)
	if seenUpToID > 0 {
		return m.MarkReadBy(msg.GuidanceID, msg.AuthorRole, seenUpToID, at)
	}
	return nil
}
func (m *memConsultation) ListMessages(guidanceID uint) ([]models.GuidanceMessage, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var out []models.GuidanceMessage
	for _, msg := range m.messages {
		if msg.GuidanceID == guidanceID {
			out = append(out, msg)
		}
	}
	if m.afterList != nil {
		m.afterList()
		m.afterList = nil
	}
	return out, nil
}
func (m *memConsultation) MarkReadBy(guidanceID uint, readerRole string, upToID uint, at time.Time) error {
	for i := range m.messages {
		msg := &m.messages[i]
		if msg.GuidanceID == guidanceID && msg.AuthorRole != readerRole && msg.ReadAt == nil && msg.ID <= upToID {
			msg.ReadAt = &at
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
func (m *memConsultation) CountOpenQuestions(studentUserID uint) (int64, error) {
	var n int64
	for _, g := range m.guidances {
		if g.StudentUserID != studentUserID || g.Kind != models.GuidanceKindQuestion {
			continue
		}
		answered := false
		for _, msg := range m.messages {
			if msg.GuidanceID == g.ID && msg.AuthorRole == models.GuidanceAuthorTeacher {
				answered = true
			}
		}
		if !answered {
			n++
		}
	}
	return n, nil
}
func (m *memConsultation) ListStudentThreads(studentUserID uint, before *repositories.StudentThreadCursor, limit int) ([]repositories.StudentThreadRow, error) {
	m.gotLimit = limit
	var out []repositories.StudentThreadRow
	for id := uint(len(m.guidances)); id >= 1 && len(out) < limit; id-- {
		g := m.guidances[id]
		if g.StudentUserID == studentUserID && (before == nil || id < before.ID) {
			out = append(out, repositories.StudentThreadRow{GuidanceID: id, Kind: g.Kind, Message: g.Message})
		}
	}
	return out, nil
}
func (m *memConsultation) TeacherInbox(_ *uint, beforeID uint, limit int) ([]repositories.ConsultationInboxRow, error) {
	m.gotBefore, m.gotLimit = beforeID, limit
	return m.inbox[:min(limit, len(m.inbox))], nil
}

func TestConsultation_学生の返信と先生の既読(t *testing.T) {
	store := newMem()
	s := NewConsultationService(store)

	// 他人の案内には返信できない（見つからない扱い）
	if _, err := s.StudentReply(99, 1, "質問です"); !errors.Is(err, ErrConsultationNotFound) {
		t.Fatalf("他人の案内: err = %v", err)
	}
	view, err := s.StudentReply(10, 1, "どこを直せばよいですか？")
	if err != nil || len(view.Messages) != 1 || view.Message != "履歴書を見直しましょう" {
		t.Fatalf("view = %+v err = %v", view, err)
	}

	g, _ := s.FindGuidance(1)
	// 先生が画面で見ていた最後の発言（ID 1）までを、返信と一緒に既読にする
	if _, err := s.TeacherReply(20, g, "自己PRを具体的にしましょう", 1); err != nil {
		t.Fatalf("teacher reply: %v", err)
	}
	if store.messages[0].ReadAt == nil {
		t.Fatal("先生が返信したのに、見ていた学生の発言が未読のまま")
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

// 本文の長さの境界: 前後の空白を除いて 1000 文字まで。文字数は Unicode のコードポイントで数える。
func TestConsultation_本文の長さの境界(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want error
	}{
		{"空白だけ", " \n\t ", ErrConsultationEmpty},
		{"1000文字", strings.Repeat("あ", 1000), nil},
		{"1001文字", strings.Repeat("あ", 1001), ErrConsultationTooLong},
		{"前後の空白を除くと1000文字", "  " + strings.Repeat("あ", 1000) + "\n", nil},
		{"絵文字1000個（UTF-16 では2000）", strings.Repeat("😀", 1000), nil},
	} {
		_, err := NewConsultationService(newMem()).StudentReply(10, 1, tt.body)
		if !errors.Is(err, tt.want) && !(tt.want == nil && err == nil) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}
}

// 先生が画面を開いたあと、返信する前に学生が追記した発言は、先生が見ていないので未読のまま残す。
func TestConsultation_表示のあとに届いた発言は既読にしない(t *testing.T) {
	store := newMem()
	s := NewConsultationService(store)
	_, _ = s.StudentReply(10, 1, "1つ目の質問")
	g, _ := s.FindGuidance(1)

	// 先生がスレッドを開く。取得の直後に学生が2つ目を送る
	store.afterList = func() {
		_ = store.CreateMessage(&models.GuidanceMessage{GuidanceID: 1, AuthorRole: models.GuidanceAuthorStudent, Body: "2つ目"})
	}
	view, err := s.TeacherThread(g)
	if err != nil || len(view.Messages) != 1 {
		t.Fatalf("view = %+v err = %v", view, err)
	}
	if store.messages[0].ReadAt == nil || store.messages[1].ReadAt != nil {
		t.Fatalf("開いたときに表示していない発言まで既読にした: %+v", store.messages)
	}

	// 画面には1つ目までしか出ていない状態で返信する
	if _, err := s.TeacherReply(20, g, "答えます", view.Messages[0].ID); err != nil {
		t.Fatal(err)
	}
	if store.messages[1].ReadAt != nil {
		t.Fatal("返信で、先生が見ていない2つ目の発言まで既読にした")
	}
}

// 返信は保存できたが、スレッドの読み直しに失敗したら、エラーにせず stale で返す（再送で二重にしない）。
func TestConsultation_保存後の読み直しに失敗しても送信は成功(t *testing.T) {
	store := newMem()
	g, _ := NewConsultationService(store).FindGuidance(1)
	store.listErr = errors.New("db")
	view, err := NewConsultationService(store).TeacherReply(20, g, "答えます", 0)
	if err != nil || !view.Stale || len(view.Messages) != 1 || view.Messages[0].Body != "答えます" {
		t.Fatalf("view = %+v err = %v", view, err)
	}
	if len(store.messages) != 1 {
		t.Fatalf("保存されていない: %+v", store.messages)
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

// 先生がまだ答えていない質問は MaxOpenQuestions 件まで。答えが付けば、また質問できる。
func TestConsultation_未回答の質問の上限(t *testing.T) {
	store := newMem()
	s := NewConsultationService(store)
	var first uint
	for i := range MaxOpenQuestions {
		v, err := s.Ask(10, "質問")
		if err != nil {
			t.Fatalf("%d件目: %v", i+1, err)
		}
		if i == 0 {
			first = v.GuidanceID
		}
	}
	if _, err := s.Ask(10, "質問"); !errors.Is(err, ErrConsultationTooManyOpen) {
		t.Fatalf("上限を超えた: err = %v", err)
	}
	// ほかの学生は影響を受けない
	if _, err := s.Ask(11, "質問"); err != nil {
		t.Fatalf("ほかの学生: %v", err)
	}
	g, _ := s.FindGuidance(first)
	if _, err := s.TeacherReply(20, g, "答えます", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Ask(10, "質問"); err != nil {
		t.Fatalf("答えが付いたのに質問できない: %v", err)
	}
}

func TestConsultation_Inboxのページング(t *testing.T) {
	store := newMem()
	store.inbox = []repositories.ConsultationInboxRow{
		{GuidanceID: 3, LastMessageID: 30}, {GuidanceID: 2, LastMessageID: 20}, {GuidanceID: 1, LastMessageID: 10},
	}
	page, err := NewConsultationService(store).Inbox(nil, 40, 2)
	if err != nil || len(page.Items) != 2 || page.NextBeforeID != 20 {
		t.Fatalf("page = %+v err = %v", page, err)
	}
	if store.gotBefore != 40 || store.gotLimit != 3 {
		t.Fatalf("before=%d limit=%d（続きの有無を見るため1件多く取る）", store.gotBefore, store.gotLimit)
	}
	page, _ = NewConsultationService(store).Inbox(nil, 0, 3)
	if len(page.Items) != 3 || page.NextBeforeID != 0 {
		t.Fatalf("最後のページなのに続きがある: %+v", page)
	}
}

func TestConsultation_学生の相談一覧のページング(t *testing.T) {
	store := newMem()
	s := NewConsultationService(store)
	for range 2 {
		_, _ = s.Ask(10, "質問")
	}
	page, err := s.StudentThreads(10, nil, 2)
	if err != nil || len(page.Items) != 2 || page.Next == nil || page.Next.ID != 2 {
		t.Fatalf("page = %+v err = %v", page, err)
	}
	page, _ = s.StudentThreads(10, page.Next, 2)
	if len(page.Items) != 1 || page.Items[0].GuidanceID != 1 || page.Next != nil {
		t.Fatalf("2ページ目 = %+v", page)
	}
}
