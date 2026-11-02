package ability

import (
	"errors"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrNotFound         = errors.New("ability assessment not found")
	ErrAlreadySubmitted = repositories.ErrAbilityAlreadySubmitted
	ErrInvalidChoice    = errors.New("invalid choice")
	// ErrDeadlinePassed は制限時間を過ぎた受検への操作。HTTP では 410 にする。
	ErrDeadlinePassed = errors.New("ability assessment deadline passed")
)

// Repository はサービスが使う永続化の面。
type Repository interface {
	ListTemplates() ([]models.AbilityTemplate, error)
	FindTemplate(id uint) (*models.AbilityTemplate, error)
	ListQuestions(templateID uint, version int) ([]models.AbilityQuestion, error)
	ListSessionsByUser(userID uint) ([]models.AbilitySession, error)
	FindInProgressSession(userID, templateID uint) (*models.AbilitySession, error)
	CreateSession(s *models.AbilitySession) error
	FindSessionByPublicID(userID uint, publicID string) (*models.AbilitySession, error)
	UpsertAnswer(sessionID, questionID uint, choice int) error
	ListAnswers(sessionID uint) ([]models.AbilityAnswer, error)
	CountAnswersBySessions(sessionIDs []uint) (map[uint]int, error)
	Submit(s *models.AbilitySession, score func(answers map[uint]int) (map[string]any, error)) error
	FindResult(sessionID uint) (map[string]any, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// 状態は記号と文言の両方で画面に出す。ここでは機械可読な値だけ返す。
const (
	StatusNotStarted = "not_started"
	StatusInProgress = models.AbilityStatusInProgress
	StatusSubmitted  = models.AbilityStatusSubmitted
	// StatusExpired は受検中のまま制限時間を過ぎたもの。再開させない。
	StatusExpired = "expired"
)

type TemplateStatus struct {
	TemplateID       uint       `json:"template_id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	QuestionCount    int        `json:"question_count"`
	TimeLimitSeconds *int       `json:"time_limit_seconds,omitempty"`
	Status           string     `json:"status"`
	SessionID        string     `json:"session_id,omitempty"`
	AnsweredCount    int        `json:"answered_count"`
	SubmittedAt      *time.Time `json:"submitted_at,omitempty"`
}

// AbilityQuestionView は受検中に返す問題。
//
// models.AbilityQuestion の CorrectChoice をここに持たせてはいけない。
// 正解をレスポンスへ出さないことを「詰め替えを忘れない」ではなく型で成立させるため、
// 専用の型にして必要なフィールドだけを並べている。
type AbilityQuestionView struct {
	ID       uint     `json:"id"`
	Position int      `json:"position"`
	Section  string   `json:"section"`
	Body     string   `json:"body"`
	Choices  []string `json:"choices"`
}

type SessionView struct {
	SessionID string                `json:"session_id"`
	Title     string                `json:"title"`
	Status    string                `json:"status"`
	Questions []AbilityQuestionView `json:"questions"`
	// Answers は question_id -> 選択肢の添字。再開時に途中から続けるため。
	Answers map[uint]int `json:"answers"`
	// DeadlineAt は締め切り。表示用。判定はサーバが行う。
	DeadlineAt *time.Time `json:"deadline_at,omitempty"`
	// RemainingSeconds は残り秒。負にはしない。
	RemainingSeconds *int `json:"remaining_seconds,omitempty"`
}

// List は学生の能力検査一覧。未受検を先に、提出済みを後に並べる。
func (s *Service) List(userID uint) ([]TemplateStatus, error) {
	templates, err := s.repo.ListTemplates()
	if err != nil {
		return nil, err
	}
	sessions, err := s.repo.ListSessionsByUser(userID)
	if err != nil {
		return nil, err
	}
	latest := map[uint]models.AbilitySession{}
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

	now := s.now()
	var open, done []TemplateStatus
	for _, t := range templates {
		version := t.Version
		ss, hasSession := latest[t.ID]
		if hasSession {
			version = ss.TemplateVersion
		}
		qs, err := s.repo.ListQuestions(t.ID, version)
		if err != nil {
			return nil, err
		}
		st := TemplateStatus{
			TemplateID: t.ID, Title: t.Title, Description: t.Description,
			QuestionCount: len(qs), TimeLimitSeconds: t.TimeLimitSeconds, Status: StatusNotStarted,
		}
		if hasSession {
			st.Status = ss.Status
			st.SessionID = ss.PublicID
			st.AnsweredCount = counts[ss.ID]
			st.SubmittedAt = ss.SubmittedAt
			if ss.Status == models.AbilityStatusInProgress && expired(&ss, now) {
				st.Status = StatusExpired
			}
		}
		if st.Status == StatusSubmitted {
			done = append(done, st)
		} else {
			open = append(open, st)
		}
	}
	return append(open, done...), nil
}

// Start は受検を始める。同じ検査の受検中があればそれを返す（やり直しで回答を失わない）。
// 制限時間を過ぎた受検中は再開させない（ErrDeadlinePassed）。
func (s *Service) Start(userID, templateID uint) (*SessionView, error) {
	t, err := s.repo.FindTemplate(templateID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sess, err := s.repo.FindInProgressSession(userID, templateID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := s.now()
		sess = &models.AbilitySession{
			PublicID: uuid.NewString(), UserID: userID, TemplateID: t.ID,
			TemplateVersion: t.Version, Status: models.AbilityStatusInProgress,
			DeadlineAt: deadlineFrom(t, now),
		}
		err = s.repo.CreateSession(sess)
		if errors.Is(err, repositories.ErrAbilitySessionExists) {
			// 別タブ・並行POSTが先に作った。そちらを使う（回答が2つに割れないように）
			sess, err = s.repo.FindInProgressSession(userID, templateID)
		}
	}
	if err != nil {
		return nil, err
	}
	if expired(sess, s.now()) {
		return nil, ErrDeadlinePassed
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
	// 提出済みは締め切り後でも読める（結果を見るため）。受検中だけ締め切りで止める。
	if sess.Status == models.AbilityStatusInProgress && expired(sess, s.now()) {
		return nil, ErrDeadlinePassed
	}
	return s.view(sess, t.Title)
}

// SaveAnswer は1問の自動保存。同じ選択の再送は何度来ても結果が変わらない。
//
// 状態と締め切りをここでも見るのは、エラーを区別して返すため（409 と 410）。
// 実際に書くかどうかは repo.UpsertAnswer の条件付き書き込みが決める。
// ここの確認だけに頼ると、確認してから書くまでの間に提出が確定したり
// 締め切りを過ぎたりしたとき、採点に入らない回答が保存される。
func (s *Service) SaveAnswer(userID uint, publicID string, questionID uint, choice int) error {
	sess, err := s.session(userID, publicID)
	if err != nil {
		return err
	}
	if sess.Status != models.AbilityStatusInProgress {
		return ErrAlreadySubmitted
	}
	// 締め切り後の回答は受け付けない。クライアントのタイマーを信じない。
	if expired(sess, s.now()) {
		return ErrDeadlinePassed
	}
	qs, err := s.repo.ListQuestions(sess.TemplateID, sess.TemplateVersion)
	if err != nil {
		return err
	}
	for _, q := range qs {
		if q.ID != questionID {
			continue
		}
		ok, err := ValidChoice(q, choice)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInvalidChoice
		}
		err = s.repo.UpsertAnswer(sess.ID, questionID, choice)
		if errors.Is(err, repositories.ErrAbilityAnswerRejected) {
			// 書く直前に状態が変わっていた。読み直して 409 / 410 を決める。
			return s.rejectedReason(userID, publicID)
		}
		return err
	}
	// 別の検査・別の版の問への回答は受け付けない
	return ErrInvalidChoice
}

// rejectedReason は条件付き書き込みが通らなかった理由を、セッションを読み直して決める。
// 読み直せないときは締め切り扱いにする（受け付けなかったことは確かなので、
// 「保存できた」と誤解させるより「締め切った」と伝える方が実害が小さい）。
func (s *Service) rejectedReason(userID uint, publicID string) error {
	sess, err := s.session(userID, publicID)
	if err != nil {
		return ErrDeadlinePassed
	}
	if sess.Status != models.AbilityStatusInProgress {
		return ErrAlreadySubmitted
	}
	return ErrDeadlinePassed
}

// Submit は回答を採点して確定する。未回答は誤答扱いで、出題数には数える。
//
// 締め切りを過ぎていても提出は受け付ける。締め切り時点の回答で採点されるため、
// ここで弾くと「時間切れで結果が一切出ない」になり、受検した意味が無くなる。
//
// 採点は repo.Submit のトランザクションの中で行う。提出済みへ切り替えたあとに
// 回答を読むので、採点した集合と保存されている回答が必ず一致する
// （ここで先に採点すると、その後に確定した回答が保存だけされて採点に入らない）。
func (s *Service) Submit(userID uint, publicID string) (map[string]SectionScore, error) {
	sess, err := s.session(userID, publicID)
	if err != nil {
		return nil, err
	}
	if sess.Status != models.AbilityStatusInProgress {
		return nil, ErrAlreadySubmitted
	}
	qs, err := s.repo.ListQuestions(sess.TemplateID, sess.TemplateVersion)
	if err != nil {
		return nil, err
	}

	var scores map[string]SectionScore
	err = s.repo.Submit(sess, func(answers map[uint]int) (map[string]any, error) {
		sc, err := ScoreAnswers(qs, answers)
		if err != nil {
			return nil, err
		}
		scores = sc
		payload := make(map[string]any, len(sc))
		for k, v := range sc {
			payload[k] = v
		}
		return payload, nil
	})
	if err != nil {
		return nil, err
	}
	return scores, nil
}

type ResultView struct {
	SessionID string                  `json:"session_id"`
	Title     string                  `json:"title"`
	Sections  map[string]SectionScore `json:"sections"`
	// Order は表示順。map の順序に依存させない。
	Order       []string   `json:"order"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
}

// Result は提出済みの結果。未提出は ErrNotFound（まだ結果が無い）。
func (s *Service) Result(userID uint, publicID string) (*ResultView, error) {
	sess, err := s.session(userID, publicID)
	if err != nil {
		return nil, err
	}
	if sess.Status != models.AbilityStatusSubmitted {
		return nil, ErrNotFound
	}
	raw, err := s.repo.FindResult(sess.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t, err := s.repo.FindTemplate(sess.TemplateID)
	if err != nil {
		return nil, err
	}
	sections := map[string]SectionScore{}
	for _, name := range models.AbilitySections {
		v, ok := raw[name]
		if !ok {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		sections[name] = SectionScore{Correct: intOf(m["correct"]), Total: intOf(m["total"])}
	}
	order := make([]string, 0, len(sections))
	for _, name := range models.AbilitySections {
		if _, ok := sections[name]; ok {
			order = append(order, name)
		}
	}
	return &ResultView{SessionID: sess.PublicID, Title: t.Title, Sections: sections, Order: order, SubmittedAt: sess.SubmittedAt}, nil
}

func (s *Service) session(userID uint, publicID string) (*models.AbilitySession, error) {
	sess, err := s.repo.FindSessionByPublicID(userID, publicID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return sess, err
}

func (s *Service) answers(sessionID uint) (map[uint]int, error) {
	rows, err := s.repo.ListAnswers(sessionID)
	if err != nil {
		return nil, err
	}
	m := make(map[uint]int, len(rows))
	for _, a := range rows {
		m[a.QuestionID] = a.Choice
	}
	return m, nil
}

func (s *Service) view(sess *models.AbilitySession, title string) (*SessionView, error) {
	qs, err := s.repo.ListQuestions(sess.TemplateID, sess.TemplateVersion)
	if err != nil {
		return nil, err
	}
	answers, err := s.answers(sess.ID)
	if err != nil {
		return nil, err
	}
	v := &SessionView{
		SessionID: sess.PublicID, Title: title, Status: sess.Status,
		Questions: make([]AbilityQuestionView, 0, len(qs)), Answers: answers,
		DeadlineAt: sess.DeadlineAt,
	}
	for _, q := range qs {
		cs, err := ChoicesOf(q)
		if err != nil {
			return nil, err
		}
		v.Questions = append(v.Questions, AbilityQuestionView{
			ID: q.ID, Position: q.Position, Section: q.Section, Body: q.Body, Choices: cs,
		})
	}
	if sess.DeadlineAt != nil {
		left := int(sess.DeadlineAt.Sub(s.now()).Seconds())
		if left < 0 {
			left = 0
		}
		v.RemainingSeconds = &left
	}
	return v, nil
}

// deadlineFrom は開始時刻から締め切りを決める。制限時間が無ければ NULL。
func deadlineFrom(t *models.AbilityTemplate, now time.Time) *time.Time {
	if t.TimeLimitSeconds == nil || *t.TimeLimitSeconds <= 0 {
		return nil
	}
	d := now.Add(time.Duration(*t.TimeLimitSeconds) * time.Second)
	return &d
}

// expired は締め切りを過ぎているか。締め切りが無ければ常に false。
func expired(s *models.AbilitySession, now time.Time) bool {
	return s.DeadlineAt != nil && now.After(*s.DeadlineAt)
}

func intOf(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
