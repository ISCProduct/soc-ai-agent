package assessment

import (
	"context"
	"errors"
	"log"
	"time"

	"Backend/domain/valueobject"
	"Backend/internal/config"
	"Backend/internal/models"
	"Backend/internal/repositories"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrNotFound         = errors.New("assessment not found")
	ErrAlreadySubmitted = repositories.ErrAssessmentAlreadySubmitted
	ErrIncomplete       = errors.New("assessment has unanswered questions")
	ErrInvalidAnswer    = errors.New("invalid answer")
)

// Repository はサービスが使う永続化の面。
type Repository interface {
	ListTemplates(schoolID *uint) ([]models.AssessmentTemplate, error)
	FindTemplate(id uint) (*models.AssessmentTemplate, error)
	FindVisibleTemplate(id uint, schoolID *uint) (*models.AssessmentTemplate, error)
	FindUserSchoolID(userID uint) (*uint, error)
	ListQuestions(templateID uint, version int) ([]models.AssessmentQuestion, error)
	ListSessionsByUser(userID uint) ([]models.AssessmentSession, error)
	FindInProgressSession(userID, templateID uint) (*models.AssessmentSession, error)
	CreateSession(s *models.AssessmentSession) error
	FindSessionByPublicID(userID uint, publicID string) (*models.AssessmentSession, error)
	UpsertAnswer(sessionID, questionID uint, value int) error
	ListAnswers(sessionID uint) ([]models.AssessmentAnswer, error)
	CountAnswersBySessions(sessionIDs []uint) (map[uint]int, error)
	Submit(s *models.AssessmentSession, scores map[string]float64, scoreSessionID string, writeScores bool) error
	FindResult(sessionID uint) (map[string]float64, error)
}

// MatchingRunner は提出後にマッチングを再計算する面（合流を反映するため）。
type MatchingRunner interface {
	CalculateMatching(ctx context.Context, userID uint, sessionID string) error
}

// ChatSessionFinder は合流先のチャット診断セッションを探す面。
type ChatSessionFinder interface {
	FindLatestDiagnosisSessionID(userID uint) (string, error)
}

type Service struct {
	repo     Repository
	matching MatchingRunner
	chat     ChatSessionFinder
}

func NewService(repo Repository, matching MatchingRunner, chat ChatSessionFinder) *Service {
	return &Service{repo: repo, matching: matching, chat: chat}
}

// 状態は記号と文言の両方で画面に出す。ここでは機械可読な値だけ返す。
const (
	StatusNotStarted = "not_started"
	StatusInProgress = models.AssessmentStatusInProgress
	StatusSubmitted  = models.AssessmentStatusSubmitted
)

type TemplateStatus struct {
	TemplateID    uint       `json:"template_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	QuestionCount int        `json:"question_count"`
	Status        string     `json:"status"`
	SessionID     string     `json:"session_id,omitempty"`
	AnsweredCount int        `json:"answered_count"`
	SubmittedAt   *time.Time `json:"submitted_at,omitempty"`
	// Scope は standard（運営標準）か school（学校独自）。学校独自は「おすすめ企業の計算には使われない」旨を画面に出す。
	Scope string `json:"scope"`
	// DueAt は受検期限。Overdue は期限を過ぎていて未提出のとき true。
	DueAt   *time.Time `json:"due_at,omitempty"`
	Overdue bool       `json:"overdue"`
}

// 検査の提供元（TemplateStatus.Scope）。
const (
	ScopeStandard = "standard"
	ScopeSchool   = "school"
)

func scopeOf(t models.AssessmentTemplate) string {
	if t.SchoolID != nil {
		return ScopeSchool
	}
	return ScopeStandard
}

// List は学生の検査一覧。未受検・受検中を先に、提出済みを後に並べる。
func (s *Service) List(userID uint) ([]TemplateStatus, error) {
	schoolID, err := s.repo.FindUserSchoolID(userID)
	if err != nil {
		return nil, err
	}
	templates, err := s.repo.ListTemplates(schoolID)
	if err != nil {
		return nil, err
	}
	sessions, err := s.repo.ListSessionsByUser(userID)
	if err != nil {
		return nil, err
	}
	// テンプレートごとに最新の受検（ListSessionsByUser は id 降順）
	latest := map[uint]models.AssessmentSession{}
	ids := []uint{}
	for _, ss := range sessions {
		if _, ok := latest[ss.TemplateID]; !ok {
			latest[ss.TemplateID] = ss
			ids = append(ids, ss.ID)
		}
	}
	counts, err := s.repo.CountAnswersBySessions(ids)
	if err != nil {
		return nil, err
	}

	var open, done []TemplateStatus
	for _, t := range templates {
		// 受検済み・受検中は開始時の版に固定されているので、その版の問題数を出す
		version := t.Version
		ss, hasSession := latest[t.ID]
		if hasSession {
			version = ss.TemplateVersion
		}
		qs, err := s.repo.ListQuestions(t.ID, version)
		if err != nil {
			return nil, err
		}
		st := TemplateStatus{TemplateID: t.ID, Title: t.Title, Description: t.Description, QuestionCount: len(qs), Status: StatusNotStarted,
			Scope: scopeOf(t), DueAt: t.DueAt}
		if hasSession {
			st.Status = ss.Status
			st.SessionID = ss.PublicID
			st.AnsweredCount = counts[ss.ID]
			st.SubmittedAt = ss.SubmittedAt
		}
		st.Overdue = isOverdue(t.DueAt, st.Status, time.Now())
		if st.Status == StatusSubmitted {
			done = append(done, st)
		} else {
			open = append(open, st)
		}
	}
	return append(open, done...), nil
}

type QuestionView struct {
	ID       uint   `json:"id"`
	Position int    `json:"position"`
	Body     string `json:"body"`
}

type SessionView struct {
	SessionID string         `json:"session_id"`
	Title     string         `json:"title"`
	Status    string         `json:"status"`
	Questions []QuestionView `json:"questions"`
	// Answers は question_id -> 1〜5。再開時に途中から続けるため。
	Answers map[uint]int `json:"answers"`
}

// Start は受検を始める。同じ検査の受検中があればそれを返す（やり直しで回答を失わない）。
func (s *Service) Start(userID, templateID uint) (*SessionView, error) {
	schoolID, err := s.repo.FindUserSchoolID(userID)
	if err != nil {
		return nil, err
	}
	// 他校の独自テンプレートは存在しない扱い（404）。
	t, err := s.repo.FindVisibleTemplate(templateID, schoolID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sess, err := s.repo.FindInProgressSession(userID, templateID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		sess = &models.AssessmentSession{
			PublicID: uuid.NewString(), UserID: userID, TemplateID: t.ID,
			TemplateVersion: t.Version, Status: models.AssessmentStatusInProgress,
		}
		err = s.repo.CreateSession(sess)
		if errors.Is(err, repositories.ErrAssessmentSessionExists) {
			// 別タブ・並行POSTが先に作った。そちらを使う（回答が2つに割れないように）
			sess, err = s.repo.FindInProgressSession(userID, templateID)
		}
	}
	if err != nil {
		return nil, err
	}
	return s.view(sess, t.Title)
}

// Get は受検の現在の状態（問題と保存済みの回答）を返す。
func (s *Service) Get(userID uint, publicID string) (*SessionView, error) {
	sess, err := s.session(userID, publicID)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.FindTemplate(sess.TemplateID)
	if err != nil {
		return nil, err
	}
	return s.view(sess, t.Title)
}

// SaveAnswer は1問の自動保存。同じ値の再送は何度来ても結果が変わらない。
func (s *Service) SaveAnswer(userID uint, publicID string, questionID uint, value int) error {
	if value < LikertMin || value > LikertMax {
		return ErrInvalidAnswer
	}
	sess, err := s.session(userID, publicID)
	if err != nil {
		return err
	}
	if sess.Status != models.AssessmentStatusInProgress {
		return ErrAlreadySubmitted
	}
	qs, err := s.repo.ListQuestions(sess.TemplateID, sess.TemplateVersion)
	if err != nil {
		return err
	}
	for _, q := range qs {
		if q.ID == questionID {
			return s.repo.UpsertAnswer(sess.ID, questionID, value)
		}
	}
	// 別の検査・別の版の問への回答は受け付けない
	return ErrInvalidAnswer
}

// Submit は全問の回答を採点して確定し、標準検査ならマッチングを再計算する。
func (s *Service) Submit(ctx context.Context, userID uint, publicID string) (map[string]float64, error) {
	sess, err := s.session(userID, publicID)
	if err != nil {
		return nil, err
	}
	if sess.Status != models.AssessmentStatusInProgress {
		return nil, ErrAlreadySubmitted
	}
	qs, err := s.repo.ListQuestions(sess.TemplateID, sess.TemplateVersion)
	if err != nil {
		return nil, err
	}
	answers, err := s.answers(sess.ID)
	if err != nil {
		return nil, err
	}
	for _, q := range qs {
		if _, ok := answers[q.ID]; !ok {
			return nil, ErrIncomplete
		}
	}
	scores, err := ScoreAnswers(qs, answers)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.FindTemplate(sess.TemplateID)
	if err != nil {
		return nil, err
	}
	// 学校独自テンプレートはマッチングに使わないので user_weight_scores へは書かない（#1675）。
	if err := s.repo.Submit(sess, scores, "assessment:"+sess.PublicID, t.IsStandard); err != nil {
		return nil, err
	}
	s.rematch(ctx, userID, sess.TemplateID)
	return scores, nil
}

// rematch は合流を有効にしているとき、最新のチャット診断でマッチングを引き直す。
// 失敗しても提出は成功のまま（スコアは保存済みで、次のチャット完了時にも再計算される）。
func (s *Service) rematch(ctx context.Context, userID, templateID uint) {
	if s.matching == nil || s.chat == nil || !config.AssessmentScoreMergeEnabled() {
		return
	}
	t, err := s.repo.FindTemplate(templateID)
	if err != nil || !t.IsStandard {
		return
	}
	sessionID, err := s.chat.FindLatestDiagnosisSessionID(userID)
	if err != nil {
		// チャット診断が無い学生はマッチングの土台が無い。検査だけでは引き直さない。
		return
	}
	if err := s.matching.CalculateMatching(ctx, userID, sessionID); err != nil {
		log.Printf("[Assessment] rematch failed user=%d session=%s: %v", userID, sessionID, err)
	}
}

type CategoryScore struct {
	Category string  `json:"category"`
	Score    float64 `json:"score"`
}

type ResultView struct {
	SessionID   string          `json:"session_id"`
	Title       string          `json:"title"`
	SubmittedAt *time.Time      `json:"submitted_at,omitempty"`
	Scores      []CategoryScore `json:"scores"`
	// Scope は standard / school。結果画面の「この結果の使われ方」を出し分ける（学校独自はマッチングに使わない）。
	Scope string `json:"scope"`
}

// Result は提出済みの結果。カテゴリは正典の順に並べる（順位付けしない）。
func (s *Service) Result(userID uint, publicID string) (*ResultView, error) {
	sess, err := s.session(userID, publicID)
	if err != nil {
		return nil, err
	}
	if sess.Status != models.AssessmentStatusSubmitted {
		return nil, ErrNotFound
	}
	t, err := s.repo.FindTemplate(sess.TemplateID)
	if err != nil {
		return nil, err
	}
	m, err := s.repo.FindResult(sess.ID)
	if err != nil {
		return nil, err
	}
	out := &ResultView{SessionID: sess.PublicID, Title: t.Title, SubmittedAt: sess.SubmittedAt, Scores: []CategoryScore{}, Scope: scopeOf(*t)}
	for _, c := range valueobject.AllWeightCategories() {
		if v, ok := m[string(c)]; ok {
			out.Scores = append(out.Scores, CategoryScore{Category: string(c), Score: v})
		}
	}
	return out, nil
}

// session は本人の受検だけを返す。受検の問題・回答・結果の入口（Get / SaveAnswer /
// Submit / Result）はすべてここを通る。
//
// 学校独自テンプレートの受検は、開始後に学校が変わったら続けさせない。
// users.school_id は本人のプロフィール更新で変わるので（auth_profile.go）、
// 本人確認だけだと転校・学校名の付け替えのあとも旧学校の問題文と結果を読めてしまう。
// 現在の所属校と一致しない学校独自テンプレートは存在しない扱い（#1675）。
func (s *Service) session(userID uint, publicID string) (*models.AssessmentSession, error) {
	sess, err := s.repo.FindSessionByPublicID(userID, publicID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t, err := s.repo.FindTemplate(sess.TemplateID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if t.SchoolID == nil {
		return sess, nil // 運営標準は学校に紐づかない
	}
	schoolID, err := s.repo.FindUserSchoolID(userID)
	if err != nil {
		return nil, err
	}
	if schoolID == nil || *schoolID != *t.SchoolID {
		return nil, ErrNotFound
	}
	return sess, nil
}

func (s *Service) answers(sessionID uint) (map[uint]int, error) {
	rows, err := s.repo.ListAnswers(sessionID)
	if err != nil {
		return nil, err
	}
	m := make(map[uint]int, len(rows))
	for _, a := range rows {
		m[a.QuestionID] = a.Value
	}
	return m, nil
}

func (s *Service) view(sess *models.AssessmentSession, title string) (*SessionView, error) {
	qs, err := s.repo.ListQuestions(sess.TemplateID, sess.TemplateVersion)
	if err != nil {
		return nil, err
	}
	answers, err := s.answers(sess.ID)
	if err != nil {
		return nil, err
	}
	v := &SessionView{SessionID: sess.PublicID, Title: title, Status: sess.Status, Questions: make([]QuestionView, 0, len(qs)), Answers: answers}
	for _, q := range qs {
		v.Questions = append(v.Questions, QuestionView{ID: q.ID, Position: q.Position, Body: q.Body})
	}
	return v, nil
}

// isOverdue は期限を過ぎていて未提出か。提出済みは期限後に出していても超過扱いにしない。
func isOverdue(due *time.Time, status string, now time.Time) bool {
	return due != nil && status != StatusSubmitted && now.After(*due)
}
