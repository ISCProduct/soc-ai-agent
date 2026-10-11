package assessment

import (
	"context"
	"errors"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"

	"gorm.io/gorm"
)

// memRepo は Repository のメモリ実装。提出の排他は本番と同じく「受検中のときだけ通す」。
type memRepo struct {
	templates map[uint]models.AssessmentTemplate
	questions []models.AssessmentQuestion
	sessions  []*models.AssessmentSession
	answers   map[uint]map[uint]int // session -> question -> value
	results   map[uint]map[string]float64
	// chatScores は提出後も消えてはいけないチャット由来のスコア
	chatScores    map[string]float64
	writtenScores map[string]map[string]float64 // scoreSessionID -> category -> score
}

func newMemRepo() *memRepo {
	return &memRepo{
		templates: map[uint]models.AssessmentTemplate{1: {ID: 1, Title: "性格検査（標準）", Version: 2, IsStandard: true}},
		questions: []models.AssessmentQuestion{
			{ID: 10, TemplateID: 1, TemplateVersion: 1, Position: 1, WeightCategory: "技術志向"}, // 旧版
			{ID: 11, TemplateID: 1, TemplateVersion: 2, Position: 1, WeightCategory: "技術志向"},
			{ID: 12, TemplateID: 1, TemplateVersion: 2, Position: 2, WeightCategory: "技術志向", ReverseScored: true},
		},
		answers:       map[uint]map[uint]int{},
		results:       map[uint]map[string]float64{},
		chatScores:    map[string]float64{"技術志向": 40},
		writtenScores: map[string]map[string]float64{},
	}
}

func (m *memRepo) ListTemplates() ([]models.AssessmentTemplate, error) {
	return []models.AssessmentTemplate{m.templates[1]}, nil
}
func (m *memRepo) FindTemplate(id uint) (*models.AssessmentTemplate, error) {
	t, ok := m.templates[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &t, nil
}
func (m *memRepo) ListQuestions(templateID uint, version int) ([]models.AssessmentQuestion, error) {
	var out []models.AssessmentQuestion
	for _, q := range m.questions {
		if q.TemplateID == templateID && q.TemplateVersion == version {
			out = append(out, q)
		}
	}
	return out, nil
}
func (m *memRepo) ListSessionsByUser(userID uint) ([]models.AssessmentSession, error) {
	var out []models.AssessmentSession
	for i := len(m.sessions) - 1; i >= 0; i-- {
		if m.sessions[i].UserID == userID {
			out = append(out, *m.sessions[i])
		}
	}
	return out, nil
}
func (m *memRepo) FindInProgressSession(userID, templateID uint) (*models.AssessmentSession, error) {
	for _, s := range m.sessions {
		if s.UserID == userID && s.TemplateID == templateID && s.Status == models.AssessmentStatusInProgress {
			return s, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memRepo) CreateSession(s *models.AssessmentSession) error {
	s.ID = uint(len(m.sessions) + 1)
	m.sessions = append(m.sessions, s)
	return nil
}
func (m *memRepo) FindSessionByPublicID(userID uint, publicID string) (*models.AssessmentSession, error) {
	for _, s := range m.sessions {
		if s.PublicID == publicID && s.UserID == userID {
			return s, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memRepo) UpsertAnswer(sessionID, questionID uint, value int) error {
	if m.answers[sessionID] == nil {
		m.answers[sessionID] = map[uint]int{}
	}
	m.answers[sessionID][questionID] = value
	return nil
}
func (m *memRepo) ListAnswers(sessionID uint) ([]models.AssessmentAnswer, error) {
	var out []models.AssessmentAnswer
	for q, v := range m.answers[sessionID] {
		out = append(out, models.AssessmentAnswer{SessionID: sessionID, QuestionID: q, Value: v})
	}
	return out, nil
}
func (m *memRepo) CountAnswersBySessions(ids []uint) (map[uint]int, error) {
	out := map[uint]int{}
	for _, id := range ids {
		out[id] = len(m.answers[id])
	}
	return out, nil
}
func (m *memRepo) Submit(s *models.AssessmentSession, scores map[string]float64, scoreSessionID string) error {
	if s.Status != models.AssessmentStatusInProgress {
		return ErrAlreadySubmitted
	}
	now := time.Now()
	s.Status, s.SubmittedAt = models.AssessmentStatusSubmitted, &now
	m.results[s.ID] = scores
	m.writtenScores[scoreSessionID] = scores
	return nil
}
func (m *memRepo) FindResult(sessionID uint) (map[string]float64, error) {
	return m.results[sessionID], nil
}

type spyMatching struct {
	calls []string
}

func (s *spyMatching) CalculateMatching(_ context.Context, _ uint, sessionID string) error {
	s.calls = append(s.calls, sessionID)
	return nil
}

type chatFinder struct{ sessionID string }

func (c chatFinder) FindLatestDiagnosisSessionID(uint) (string, error) {
	if c.sessionID == "" {
		return "", gorm.ErrRecordNotFound
	}
	return c.sessionID, nil
}

func newTestService(repo *memRepo, chat string) (*Service, *spyMatching) {
	spy := &spyMatching{}
	return NewService(repo, spy, chatFinder{sessionID: chat}), spy
}

// 中断して開き直すと、同じ受検と保存済みの回答が戻る。問題は開始時の版に固定される。
func TestService_StartResumesInProgress(t *testing.T) {
	repo := newMemRepo()
	svc, _ := newTestService(repo, "")
	first, err := svc.Start(7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveAnswer(7, first.SessionID, 11, 4); err != nil {
		t.Fatal(err)
	}
	again, err := svc.Start(7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if again.SessionID != first.SessionID || again.Answers[11] != 4 {
		t.Fatalf("再開できていない: %+v", again)
	}
	if len(again.Questions) != 2 || again.Questions[0].ID != 11 {
		t.Fatalf("開始時の版(v2)の問題だけを出す: %+v", again.Questions)
	}

	// 公開後に版が上がっても、受検中の問題は揺れない
	repo.templates[1] = models.AssessmentTemplate{ID: 1, Title: "性格検査（標準）", Version: 3, IsStandard: true}
	got, _ := svc.Get(7, first.SessionID)
	if len(got.Questions) != 2 {
		t.Fatalf("版の凍結が効いていない: %+v", got.Questions)
	}
	// 一覧の問題数も受検中の版で数える（v3 の問題数を出すと「進捗が終わらない」ように見える）
	repo.questions = append(repo.questions, models.AssessmentQuestion{ID: 13, TemplateID: 1, TemplateVersion: 3, Position: 1, WeightCategory: "技術志向"})
	list, _ := svc.List(7)
	if list[0].QuestionCount != 2 {
		t.Fatalf("一覧の問題数が受検中の版になっていない: %+v", list[0])
	}
}

// racingRepo は「未存在を確認した後に別リクエストが先に作った」状況を再現する。
type racingRepo struct {
	*memRepo
	winner *models.AssessmentSession
}

func (r *racingRepo) FindInProgressSession(userID, templateID uint) (*models.AssessmentSession, error) {
	if r.winner == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.winner, nil
}

func (r *racingRepo) CreateSession(s *models.AssessmentSession) error {
	r.winner = &models.AssessmentSession{ID: 99, PublicID: "winner", UserID: s.UserID, TemplateID: s.TemplateID, TemplateVersion: s.TemplateVersion, Status: models.AssessmentStatusInProgress}
	return repositories.ErrAssessmentSessionExists
}

// 並行して開始されたら、一意制約で負けた側は先に作られた受検を返す（二重の受検を作らない）。
func TestService_StartRaceReturnsExisting(t *testing.T) {
	svc := NewService(&racingRepo{memRepo: newMemRepo()}, nil, nil)
	got, err := svc.Start(7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "winner" {
		t.Fatalf("先に作られた受検を返していない: %+v", got)
	}
}

func TestService_SaveAnswer(t *testing.T) {
	repo := newMemRepo()
	svc, _ := newTestService(repo, "")
	s, _ := svc.Start(7, 1)

	// 同じ回答を2回送っても1件のまま（冪等）
	for range 2 {
		if err := svc.SaveAnswer(7, s.SessionID, 11, 3); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(repo.answers[1]); n != 1 || repo.answers[1][11] != 3 {
		t.Fatalf("answers = %v", repo.answers[1])
	}

	tests := []struct {
		name    string
		user    uint
		qid     uint
		value   int
		wantErr error
	}{
		{"範囲外の値", 7, 11, 6, ErrInvalidAnswer},
		{"0は未回答と区別できないので弾く", 7, 11, 0, ErrInvalidAnswer},
		{"旧版の問題", 7, 10, 3, ErrInvalidAnswer},
		{"他人の受検", 8, 11, 3, ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := svc.SaveAnswer(tt.user, s.SessionID, tt.qid, tt.value); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_Submit(t *testing.T) {
	t.Setenv("ASSESSMENT_SCORE_MERGE", "")
	repo := newMemRepo()
	svc, spy := newTestService(repo, "session_chat_1")
	s, _ := svc.Start(7, 1)
	_ = svc.SaveAnswer(7, s.SessionID, 11, 5)

	if _, err := svc.Submit(context.Background(), 7, s.SessionID); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("未回答があるのに提出できた: %v", err)
	}

	_ = svc.SaveAnswer(7, s.SessionID, 12, 1)
	scores, err := svc.Submit(context.Background(), 7, s.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if scores["技術志向"] != 100 {
		t.Fatalf("scores = %v", scores)
	}
	if got := repo.writtenScores["assessment:"+s.SessionID]; got["技術志向"] != 100 {
		t.Fatalf("検査用のセッションIDで書いていない: %v", repo.writtenScores)
	}
	if repo.chatScores["技術志向"] != 40 {
		t.Fatal("チャット由来のスコアが変わった")
	}
	if len(spy.calls) != 1 || spy.calls[0] != "session_chat_1" {
		t.Fatalf("最新のチャット診断で再マッチングしていない: %v", spy.calls)
	}

	// 二重提出・提出後の回答は 409 相当
	if _, err := svc.Submit(context.Background(), 7, s.SessionID); !errors.Is(err, ErrAlreadySubmitted) {
		t.Fatalf("二重提出が通った: %v", err)
	}
	if err := svc.SaveAnswer(7, s.SessionID, 11, 1); !errors.Is(err, ErrAlreadySubmitted) {
		t.Fatalf("提出後に回答を変えられた: %v", err)
	}

	res, err := svc.Result(7, s.SessionID)
	if err != nil || len(res.Scores) != 1 || res.Scores[0].Category != "技術志向" {
		t.Fatalf("result = %+v err %v", res, err)
	}
	list, _ := svc.List(7)
	if list[0].Status != StatusSubmitted {
		t.Fatalf("list = %+v", list)
	}
}

func TestService_SubmitRematch(t *testing.T) {
	tests := []struct {
		name      string
		env       string
		chat      string
		wantCalls int
	}{
		{"合流を切ったら再計算しない", "false", "session_chat_1", 0},
		{"チャット診断が無ければ再計算しない", "", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ASSESSMENT_SCORE_MERGE", tt.env)
			repo := newMemRepo()
			svc, spy := newTestService(repo, tt.chat)
			s, _ := svc.Start(7, 1)
			_ = svc.SaveAnswer(7, s.SessionID, 11, 5)
			_ = svc.SaveAnswer(7, s.SessionID, 12, 5)
			if _, err := svc.Submit(context.Background(), 7, s.SessionID); err != nil {
				t.Fatal(err)
			}
			if len(spy.calls) != tt.wantCalls {
				t.Fatalf("calls = %v", spy.calls)
			}
		})
	}
}

// 一覧は未受検・受検中を先に、提出済みを後に出す。結果は提出前に見られない。
func TestService_ListAndResultBeforeSubmit(t *testing.T) {
	repo := newMemRepo()
	svc, _ := newTestService(repo, "")
	list, err := svc.List(7)
	if err != nil || len(list) != 1 || list[0].Status != StatusNotStarted || list[0].QuestionCount != 2 {
		t.Fatalf("list = %+v err %v", list, err)
	}
	s, _ := svc.Start(7, 1)
	_ = svc.SaveAnswer(7, s.SessionID, 11, 2)
	list, _ = svc.List(7)
	if list[0].Status != StatusInProgress || list[0].AnsweredCount != 1 {
		t.Fatalf("list = %+v", list)
	}
	if _, err := svc.Result(7, s.SessionID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("提出前に結果が見えた: %v", err)
	}
}
