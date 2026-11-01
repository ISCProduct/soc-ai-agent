package ability

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"

	"gorm.io/gorm"
)

// memRepo は Repository のメモリ実装。提出の排他は本番と同じく「受検中のときだけ通す」。
type memRepo struct {
	templates map[uint]models.AbilityTemplate
	questions []models.AbilityQuestion
	sessions  []*models.AbilitySession
	answers   map[uint]map[uint]int // session -> question -> choice
	results   map[uint]map[string]any
}

const (
	testUser     = uint(7)
	testTmplID   = uint(1)
	timeLimitSec = 900
)

func newMemRepo() *memRepo {
	limit := timeLimitSec
	return &memRepo{
		templates: map[uint]models.AbilityTemplate{
			testTmplID: {ID: testTmplID, Title: "能力検査（言語・非言語）", Version: 1, TimeLimitSeconds: &limit},
		},
		questions: []models.AbilityQuestion{
			{ID: 10, TemplateID: testTmplID, TemplateVersion: 1, Position: 1, Section: models.AbilitySectionVerbal, Body: "語彙", Choices: `["a","b","c","d"]`, CorrectChoice: 2},
			{ID: 11, TemplateID: testTmplID, TemplateVersion: 1, Position: 2, Section: models.AbilitySectionNumerical, Body: "計算", Choices: `["a","b","c","d"]`, CorrectChoice: 0},
		},
		answers: map[uint]map[uint]int{},
		results: map[uint]map[string]any{},
	}
}

func (m *memRepo) ListTemplates() ([]models.AbilityTemplate, error) {
	out := make([]models.AbilityTemplate, 0, len(m.templates))
	for id := uint(1); id <= uint(len(m.templates)); id++ {
		if t, ok := m.templates[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}
func (m *memRepo) FindTemplate(id uint) (*models.AbilityTemplate, error) {
	t, ok := m.templates[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &t, nil
}
func (m *memRepo) ListQuestions(tid uint, v int) ([]models.AbilityQuestion, error) {
	var out []models.AbilityQuestion
	for _, q := range m.questions {
		if q.TemplateID == tid && q.TemplateVersion == v {
			out = append(out, q)
		}
	}
	return out, nil
}
func (m *memRepo) ListSessionsByUser(userID uint) ([]models.AbilitySession, error) {
	var out []models.AbilitySession
	for i := len(m.sessions) - 1; i >= 0; i-- {
		if m.sessions[i].UserID == userID {
			out = append(out, *m.sessions[i])
		}
	}
	return out, nil
}
func (m *memRepo) FindInProgressSession(userID, tid uint) (*models.AbilitySession, error) {
	for _, s := range m.sessions {
		if s.UserID == userID && s.TemplateID == tid && s.Status == models.AbilityStatusInProgress {
			return s, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memRepo) CreateSession(s *models.AbilitySession) error {
	if _, err := m.FindInProgressSession(s.UserID, s.TemplateID); err == nil {
		return repositories.ErrAbilitySessionExists
	}
	s.ID = uint(len(m.sessions) + 1)
	m.sessions = append(m.sessions, s)
	return nil
}
func (m *memRepo) FindSessionByPublicID(userID uint, pid string) (*models.AbilitySession, error) {
	for _, s := range m.sessions {
		if s.PublicID == pid && s.UserID == userID {
			return s, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memRepo) UpsertAnswer(sid, qid uint, choice int) error {
	if m.answers[sid] == nil {
		m.answers[sid] = map[uint]int{}
	}
	m.answers[sid][qid] = choice
	return nil
}
func (m *memRepo) ListAnswers(sid uint) ([]models.AbilityAnswer, error) {
	var out []models.AbilityAnswer
	for qid, c := range m.answers[sid] {
		out = append(out, models.AbilityAnswer{SessionID: sid, QuestionID: qid, Choice: c})
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
func (m *memRepo) Submit(s *models.AbilitySession, scores map[string]any) error {
	if s.Status != models.AbilityStatusInProgress {
		return repositories.ErrAbilityAlreadySubmitted
	}
	now := time.Now()
	s.Status = models.AbilityStatusSubmitted
	s.SubmittedAt = &now
	// JSON を経由した形で持つ（本番は json 列なので数値は float64 で戻る）。
	b, _ := json.Marshal(scores)
	var round map[string]any
	_ = json.Unmarshal(b, &round)
	m.results[s.ID] = round
	return nil
}
func (m *memRepo) FindResult(sid uint) (map[string]any, error) {
	r, ok := m.results[sid]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return r, nil
}

func newTestService(repo *memRepo, now func() time.Time) *Service {
	svc := NewService(repo)
	if now != nil {
		svc.now = now
	}
	return svc
}

// 受検中のレスポンスに正解を出さない。
//
// この検査は「ビュー型に CorrectChoice を足さない」だけでは守れない。
// models.AbilityQuestion をうっかりそのまま返す実装に変えると漏れるので、
// JSON を直接見て、正解の添字がどのキーにも現れないことを確かめる。
func TestSessionViewHidesCorrectChoice(t *testing.T) {
	repo := newMemRepo()
	// 正解の添字を他の数字と見分けられるようにする（0/1 だと position や id と紛れる）。
	repo.questions[0].CorrectChoice = 3
	repo.questions[1].CorrectChoice = 3
	svc := newTestService(repo, nil)

	view, err := svc.Start(testUser, testTmplID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(b)
	for _, key := range []string{"correct_choice", "CorrectChoice", "correct"} {
		if strings.Contains(raw, key) {
			t.Errorf("受検中のレスポンスに正解のキーが出た: %q\n%s", key, raw)
		}
	}
	// 構造としても、問題は選択肢までしか持たない
	var parsed struct {
		Questions []map[string]any `json:"questions"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Questions) != 2 {
		t.Fatalf("問題数 = %d", len(parsed.Questions))
	}
	allowed := map[string]bool{"id": true, "position": true, "section": true, "body": true, "choices": true}
	for _, q := range parsed.Questions {
		for k := range q {
			if !allowed[k] {
				t.Errorf("問題に想定外のキーがある: %q（正解が混ざる経路になる）", k)
			}
		}
	}
}

// 制限時間はサーバが決める。開始時に締め切りを凍結し、残り秒は表示用に返す。
func TestStartFreezesDeadline(t *testing.T) {
	base := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	repo := newMemRepo()
	svc := newTestService(repo, func() time.Time { return base })

	view, err := svc.Start(testUser, testTmplID)
	if err != nil {
		t.Fatal(err)
	}
	if view.DeadlineAt == nil || !view.DeadlineAt.Equal(base.Add(timeLimitSec*time.Second)) {
		t.Fatalf("締め切り = %v, want %v", view.DeadlineAt, base.Add(timeLimitSec*time.Second))
	}
	if view.RemainingSeconds == nil || *view.RemainingSeconds != timeLimitSec {
		t.Errorf("残り秒 = %v, want %d", view.RemainingSeconds, timeLimitSec)
	}
}

// 制限時間を過ぎたら、再開も追加の回答も受け付けない（クライアントのタイマーを信じない）。
func TestDeadlineIsEnforcedServerSide(t *testing.T) {
	base := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	after := base.Add((timeLimitSec + 1) * time.Second)

	setup := func() (*Service, *memRepo, string, *time.Time) {
		repo := newMemRepo()
		clock := base
		svc := newTestService(repo, func() time.Time { return clock })
		v, err := svc.Start(testUser, testTmplID)
		if err != nil {
			t.Fatalf("開始に失敗: %v", err)
		}
		return svc, repo, v.SessionID, &clock
	}

	// 前提: 締め切り前は読めて答えられる（この確認が無いと下の4件は常に通る）
	svc, _, pid, clock := setup()
	if _, err := svc.Get(testUser, pid); err != nil {
		t.Fatalf("締め切り前に読めない: %v", err)
	}
	if err := svc.SaveAnswer(testUser, pid, 10, 2); err != nil {
		t.Fatalf("締め切り前に答えられない: %v", err)
	}
	_ = clock

	tests := []struct {
		name string
		call func(svc *Service, pid string) error
	}{
		{"状態の取得", func(s *Service, p string) error { _, err := s.Get(testUser, p); return err }},
		{"回答の保存", func(s *Service, p string) error { return s.SaveAnswer(testUser, p, 10, 2) }},
		{"再開(Start)", func(s *Service, p string) error { _, err := s.Start(testUser, testTmplID); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMemRepo()
			clock := base
			svc := newTestService(repo, func() time.Time { return clock })
			v, err := svc.Start(testUser, testTmplID)
			if err != nil {
				t.Fatal(err)
			}
			clock = after
			if err := tt.call(svc, v.SessionID); !errors.Is(err, ErrDeadlinePassed) {
				t.Errorf("締め切り後の%sが通った err=%v want %v", tt.name, err, ErrDeadlinePassed)
			}
		})
	}
}

// 締め切り後でも提出はできる。弾くと「時間切れで結果が一切出ない」になり受検した意味が無くなる。
// 締め切り後の新しい回答は SaveAnswer が拒否しているので、点数は増えない。
func TestSubmitAllowedAfterDeadline(t *testing.T) {
	base := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	repo := newMemRepo()
	clock := base
	svc := newTestService(repo, func() time.Time { return clock })

	v, err := svc.Start(testUser, testTmplID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveAnswer(testUser, v.SessionID, 10, 2); err != nil { // 正解
		t.Fatal(err)
	}
	clock = base.Add((timeLimitSec + 1) * time.Second)

	scores, err := svc.Submit(testUser, v.SessionID)
	if err != nil {
		t.Fatalf("締め切り後の提出が通らない: %v", err)
	}
	want := map[string]SectionScore{"verbal": {1, 1}, "numerical": {0, 1}}
	if !reflect.DeepEqual(scores, want) {
		t.Errorf("得点 = %v, want %v", scores, want)
	}
}

// 二重提出は片方だけ通す。
func TestSubmitTwiceConflicts(t *testing.T) {
	repo := newMemRepo()
	svc := newTestService(repo, nil)
	v, err := svc.Start(testUser, testTmplID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(testUser, v.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(testUser, v.SessionID); !errors.Is(err, ErrAlreadySubmitted) {
		t.Errorf("2回目の提出 err=%v want %v", err, ErrAlreadySubmitted)
	}
}

// 他人の受検・存在しない受検は見つからない扱い。
func TestSessionIsOwnerOnly(t *testing.T) {
	repo := newMemRepo()
	svc := newTestService(repo, nil)
	v, err := svc.Start(testUser, testTmplID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(testUser+1, v.SessionID); !errors.Is(err, ErrNotFound) {
		t.Errorf("他人の受検が読めた err=%v", err)
	}
	if _, err := svc.Get(testUser, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("存在しない受検が読めた err=%v", err)
	}
}

// 中断して開き直すと、同じ受検と保存済みの回答が戻る。
func TestStartResumesInProgress(t *testing.T) {
	repo := newMemRepo()
	svc := newTestService(repo, nil)
	first, err := svc.Start(testUser, testTmplID)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveAnswer(testUser, first.SessionID, 11, 3); err != nil {
		t.Fatal(err)
	}
	second, err := svc.Start(testUser, testTmplID)
	if err != nil {
		t.Fatal(err)
	}
	if second.SessionID != first.SessionID {
		t.Errorf("別の受検が作られた %q -> %q", first.SessionID, second.SessionID)
	}
	if second.Answers[11] != 3 {
		t.Errorf("保存済みの回答が戻らない: %v", second.Answers)
	}
}

// 選択肢の範囲外・別の検査の問は保存しない。
func TestSaveAnswerRejectsInvalid(t *testing.T) {
	repo := newMemRepo()
	svc := newTestService(repo, nil)
	v, err := svc.Start(testUser, testTmplID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		qid    uint
		choice int
	}{
		{"選択肢より大きい", 10, 4},
		{"負の添字", 10, -1},
		{"存在しない問", 999, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := svc.SaveAnswer(testUser, v.SessionID, tt.qid, tt.choice); !errors.Is(err, ErrInvalidChoice) {
				t.Errorf("err=%v want %v", err, ErrInvalidChoice)
			}
		})
	}
}

// 一覧は未受検を先に出し、受検中のまま締め切りを過ぎたものは expired として示す。
func TestListMarksExpired(t *testing.T) {
	base := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	repo := newMemRepo()
	clock := base
	svc := newTestService(repo, func() time.Time { return clock })

	rows, err := svc.List(testUser)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Status != StatusNotStarted {
		t.Fatalf("未受検の一覧 = %+v", rows)
	}
	if rows[0].QuestionCount != 2 {
		t.Errorf("問題数 = %d, want 2", rows[0].QuestionCount)
	}
	if rows[0].TimeLimitSeconds == nil || *rows[0].TimeLimitSeconds != timeLimitSec {
		t.Errorf("制限時間 = %v", rows[0].TimeLimitSeconds)
	}

	if _, err := svc.Start(testUser, testTmplID); err != nil {
		t.Fatal(err)
	}
	clock = base.Add((timeLimitSec + 1) * time.Second)
	rows, err = svc.List(testUser)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Status != StatusExpired {
		t.Errorf("締め切り後の状態 = %q, want %q", rows[0].Status, StatusExpired)
	}
}

// 結果は提出後にだけ出る。section 別の正答数と出題数で、順位や偏差値は持たない。
func TestResult(t *testing.T) {
	repo := newMemRepo()
	svc := newTestService(repo, nil)
	v, err := svc.Start(testUser, testTmplID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Result(testUser, v.SessionID); !errors.Is(err, ErrNotFound) {
		t.Errorf("提出前に結果が出た err=%v", err)
	}
	if err := svc.SaveAnswer(testUser, v.SessionID, 10, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(testUser, v.SessionID); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Result(testUser, v.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Sections, map[string]SectionScore{"verbal": {1, 1}, "numerical": {0, 1}}) {
		t.Errorf("結果 = %+v", got.Sections)
	}
	if !reflect.DeepEqual(got.Order, []string{"verbal", "numerical"}) {
		t.Errorf("表示順 = %v, want [verbal numerical]", got.Order)
	}
	b, _ := json.Marshal(got)
	for _, k := range []string{"deviation", "rank", "percentile", "偏差値"} {
		if strings.Contains(string(b), k) {
			t.Errorf("結果に %q が含まれる（順位・偏差値は出さない）", k)
		}
	}
}
