package teacher

import (
	"context"
	"errors"
	"log"
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
// 学生の学校と教員の担当校を突き合わせ、担当校でなければ「見つからない」（404）にする。

var (
	ErrConsultationNotFound    = errors.New("consultation not found")
	ErrConsultationEmpty       = errors.New("consultation message is empty")
	ErrConsultationTooLong     = errors.New("consultation message is too long")
	ErrConsultationTooManyOpen = errors.New("too many unanswered questions")
	maxConsultationBodyRunes   = 1000
)

// MaxOpenQuestions は、先生がまだ答えていない質問を学生1人が同時に持てる数。
// 1人が質問を大量に作ると、同じ学校のほかの学生の質問が受信一覧で埋もれるため。
const MaxOpenQuestions = 5

// ConsultationStore は相談の永続化（repositories.GuidanceMessageRepository が満たす）。
type ConsultationStore interface {
	FindGuidance(ctx context.Context, id uint) (*models.TeacherStudentGuidance, error)
	// CreateQuestion は未回答の質問が maxOpen 件以上なら repositories.ErrOpenQuestionLimit を返す
	CreateQuestion(ctx context.Context, studentUserID uint, body string, maxOpen int64) (*models.TeacherStudentGuidance, error)
	CreateMessage(ctx context.Context, m *models.GuidanceMessage) error
	CreateMessageAndMarkRead(ctx context.Context, m *models.GuidanceMessage, seenUpToID uint, at time.Time) error
	ListMessages(ctx context.Context, guidanceID uint) ([]models.GuidanceMessage, error)
	// MarkReadBy は messageIDs のうち、readerRole から見て相手側の未読の発言を既読にする
	MarkReadBy(ctx context.Context, guidanceID uint, readerRole string, messageIDs []uint, at time.Time) error
	UnreadForStudent(ctx context.Context, studentUserID uint) (map[uint]int, error)
	ListStudentThreads(ctx context.Context, studentUserID uint, before *repositories.StudentThreadCursor, limit int) ([]repositories.StudentThreadRow, error)
	TeacherInbox(ctx context.Context, schoolID *uint, beforeID uint, limit int) ([]repositories.ConsultationInboxRow, error)
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
	// Stale は、発言は保存できたがスレッドの取り直しに失敗したことを表す（Messages は今回の発言だけ）。
	// 送信は成功なので、画面は入力を消してスレッドを読み直す（再送で二重にしない）。
	Stale bool `json:"stale,omitempty"`
}

// validateBody は前後の空白を除き、空と 1000 文字（Unicode のコードポイント数）超えを弾く。
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

// Ask は学生から教員への質問のスレッドを作る。学生と学校の確認は呼び出し側で済ませる。
// 先生がまだ答えていない質問が MaxOpenQuestions 件あれば ErrConsultationTooManyOpen。
func (s *ConsultationService) Ask(ctx context.Context, studentUserID uint, body string) (*ThreadView, error) {
	body, err := validateBody(body)
	if err != nil {
		return nil, err
	}
	// 数えてから作るまでは、同時に送られても上限を超えないよう保存側の1トランザクションで行う
	g, err := s.store.CreateQuestion(ctx, studentUserID, body, MaxOpenQuestions)
	if errors.Is(err, repositories.ErrOpenQuestionLimit) {
		return nil, ErrConsultationTooManyOpen
	}
	if err != nil {
		return nil, err
	}
	return s.threadAfterWrite(ctx, g, models.GuidanceMessage{GuidanceID: g.ID, AuthorRole: models.GuidanceAuthorStudent, Body: body, CreatedAt: g.CreatedAt})
}

// guidanceForStudent は学生本人の案内だけを返す（他人の案内は見つからない扱い）。
func (s *ConsultationService) guidanceForStudent(ctx context.Context, studentUserID, guidanceID uint) (*models.TeacherStudentGuidance, error) {
	g, err := s.store.FindGuidance(ctx, guidanceID)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && g.StudentUserID != studentUserID) {
		return nil, ErrConsultationNotFound
	}
	return g, err
}

// FindGuidance は案内を返す（教員側の学校の確認に使う）。無ければ ErrConsultationNotFound。
func (s *ConsultationService) FindGuidance(ctx context.Context, guidanceID uint) (*models.TeacherStudentGuidance, error) {
	g, err := s.store.FindGuidance(ctx, guidanceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrConsultationNotFound
	}
	return g, err
}

func (s *ConsultationService) thread(ctx context.Context, g *models.TeacherStudentGuidance) (*ThreadView, error) {
	msgs, err := s.store.ListMessages(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	if msgs == nil {
		msgs = []models.GuidanceMessage{}
	}
	return &ThreadView{GuidanceID: g.ID, StudentUserID: g.StudentUserID, Kind: g.Kind, Message: g.Message, CreatedAt: g.CreatedAt, Messages: msgs}, nil
}

// threadAfterWrite は書き込みのあとのスレッドを返す。取り直しに失敗しても、書き込みは済んでいるので
// エラーにせず Stale を立てて返す（500 にすると、画面が入力を残して再送させ、発言が二重になる）。
func (s *ConsultationService) threadAfterWrite(ctx context.Context, g *models.TeacherStudentGuidance, written models.GuidanceMessage) (*ThreadView, error) {
	view, err := s.thread(ctx, g)
	if err == nil {
		return view, nil
	}
	log.Printf("[WARN] consultation: 発言は保存したがスレッドを読み直せなかった guidance=%d: %v", g.ID, err)
	return &ThreadView{
		GuidanceID: g.ID, StudentUserID: g.StudentUserID, Kind: g.Kind, Message: g.Message, CreatedAt: g.CreatedAt,
		Messages: []models.GuidanceMessage{written}, Stale: true,
	}, nil
}

// readThread はスレッドを返し、readerRole の側が、返した発言を読んだことにする。
// 先に一覧を取り、そこに含まれた発言の ID だけを既読にするので、取得のあとに届いた発言や、
// 採番は先でもコミットが遅れて取得に含まれなかった発言は未読のまま残る。
func (s *ConsultationService) readThread(ctx context.Context, g *models.TeacherStudentGuidance, readerRole string) (*ThreadView, error) {
	view, err := s.thread(ctx, g)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(view.Messages))
	for _, m := range view.Messages {
		if m.AuthorRole != readerRole && m.ReadAt == nil {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) > 0 {
		if err := s.store.MarkReadBy(ctx, g.ID, readerRole, ids, s.now()); err != nil {
			return nil, err
		}
	}
	return view, nil
}

// StudentThread は学生本人のスレッドを返し、表示した教員の発言を既読にする。
func (s *ConsultationService) StudentThread(ctx context.Context, studentUserID, guidanceID uint) (*ThreadView, error) {
	g, err := s.guidanceForStudent(ctx, studentUserID, guidanceID)
	if err != nil {
		return nil, err
	}
	return s.readThread(ctx, g, models.GuidanceAuthorStudent)
}

// StudentReply は学生本人のスレッドに返信する。
func (s *ConsultationService) StudentReply(ctx context.Context, studentUserID, guidanceID uint, body string) (*ThreadView, error) {
	body, err := validateBody(body)
	if err != nil {
		return nil, err
	}
	g, err := s.guidanceForStudent(ctx, studentUserID, guidanceID)
	if err != nil {
		return nil, err
	}
	m := models.GuidanceMessage{GuidanceID: g.ID, AuthorUserID: studentUserID, AuthorRole: models.GuidanceAuthorStudent, Body: body}
	if err := s.store.CreateMessage(ctx, &m); err != nil {
		return nil, err
	}
	return s.threadAfterWrite(ctx, g, m)
}

// TeacherThread はスレッドを返し、表示した学生の発言を既読にする（担当校の確認は呼び出し側で済ませる）。
func (s *ConsultationService) TeacherThread(ctx context.Context, g *models.TeacherStudentGuidance) (*ThreadView, error) {
	return s.readThread(ctx, g, models.GuidanceAuthorTeacher)
}

// TeacherReply は教員がスレッドに返信する（担当校の確認は呼び出し側で済ませる）。
// seenUpToID は、教員が返信を書いた画面に出ていた最後の発言の ID。学生の発言はそこまでを既読にする
// （画面を開いたあとに学生が追記した発言は、教員が見ていないので未読のまま残す）。0 なら既読にしない。
// 返信の保存と既読は1つのトランザクションで行う。
func (s *ConsultationService) TeacherReply(ctx context.Context, teacherUserID uint, g *models.TeacherStudentGuidance, body string, seenUpToID uint) (*ThreadView, error) {
	body, err := validateBody(body)
	if err != nil {
		return nil, err
	}
	m := models.GuidanceMessage{GuidanceID: g.ID, AuthorUserID: teacherUserID, AuthorRole: models.GuidanceAuthorTeacher, Body: body}
	if err := s.store.CreateMessageAndMarkRead(ctx, &m, seenUpToID, s.now()); err != nil {
		return nil, err
	}
	return s.threadAfterWrite(ctx, g, m)
}

// UnreadForStudent は学生の各スレッドの、教員の未読の発言の数を返す。
func (s *ConsultationService) UnreadForStudent(ctx context.Context, studentUserID uint) (map[uint]int, error) {
	return s.store.UnreadForStudent(ctx, studentUserID)
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
	LastMessageID uint      `json:"last_message_id"`
}

// InboxPage は受信一覧の1ページ。NextBeforeID が 0 でなければ、before_id にそれを渡すと続きが取れる。
type InboxPage struct {
	Items        []InboxItem `json:"items"`
	NextBeforeID uint        `json:"next_before_id,omitempty"`
}

// Inbox は学生の発言があるスレッドを、新しい順に limit 件返す。schoolID が非nilならその学校の学生だけ。
// beforeID が 0 でなければ、最後の発言の ID がそれより小さいもの（続き）だけ。
func (s *ConsultationService) Inbox(ctx context.Context, schoolID *uint, beforeID uint, limit int) (*InboxPage, error) {
	// 1件多く取り、続きがあるかを判定する
	rows, err := s.store.TeacherInbox(ctx, schoolID, beforeID, limit+1)
	if err != nil {
		return nil, err
	}
	page := &InboxPage{Items: make([]InboxItem, 0, min(len(rows), limit))}
	for i, r := range rows {
		if i == limit {
			page.NextBeforeID = page.Items[limit-1].LastMessageID
			break
		}
		page.Items = append(page.Items, InboxItem(r))
	}
	return page, nil
}

// StudentThreadItem は学生の相談一覧の1行（閉じた案内も含む）。
type StudentThreadItem struct {
	GuidanceID uint      `json:"guidance_id"`
	Kind       string    `json:"kind"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Unread     int       `json:"unread"`
}

// StudentThreadPage は学生の相談一覧の1ページ。Next が非nilなら、それを渡すと続きが取れる。
type StudentThreadPage struct {
	Items []StudentThreadItem
	Next  *repositories.StudentThreadCursor
}

// StudentThreads は学生本人の案内と質問を、新しく動いた順に limit 件返す。
// ホームの案内（10件まで・閉じたものは出ない）と違い、閉じた案内も古い相談も辿れる。
func (s *ConsultationService) StudentThreads(ctx context.Context, studentUserID uint, before *repositories.StudentThreadCursor, limit int) (*StudentThreadPage, error) {
	rows, err := s.store.ListStudentThreads(ctx, studentUserID, before, limit+1)
	if err != nil {
		return nil, err
	}
	page := &StudentThreadPage{Items: make([]StudentThreadItem, 0, min(len(rows), limit))}
	for i, r := range rows {
		if i == limit {
			last := rows[limit-1]
			page.Next = &repositories.StudentThreadCursor{UpdatedAt: last.UpdatedAt, ID: last.GuidanceID}
			break
		}
		page.Items = append(page.Items, StudentThreadItem(r))
	}
	return page, nil
}

// FormatConsultationError は入力の誤りを、利用者に見せる文言にする。
func FormatConsultationError(err error) string {
	switch {
	case errors.Is(err, ErrConsultationEmpty):
		return "メッセージを入力してください"
	case errors.Is(err, ErrConsultationTooLong):
		return "メッセージは1000文字以内にしてください"
	case errors.Is(err, ErrConsultationTooManyOpen):
		return "先生がまだ答えていない質問が5件あります。返事が届いてから、新しく質問してください"
	}
	return "送信できませんでした"
}
