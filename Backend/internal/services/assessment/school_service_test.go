package assessment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"Backend/internal/models"
	"Backend/internal/repositories"

	"gorm.io/gorm"
)

func ptr[T any](v T) *T { return &v }

// 学校独自テンプレート（school 10）を足した memRepo。問題は 2 問（技術志向 / チームワーク志向）。
func withSchoolTemplate(repo *memRepo) {
	repo.templates[2] = models.AssessmentTemplate{ID: 2, SchoolID: ptr(uint(10)), Title: "A校の検査", Version: 1}
	repo.questions = append(repo.questions,
		models.AssessmentQuestion{ID: 21, TemplateID: 2, TemplateVersion: 1, Position: 1, WeightCategory: "技術志向"},
		models.AssessmentQuestion{ID: 22, TemplateID: 2, TemplateVersion: 1, Position: 2, WeightCategory: "チームワーク志向"},
	)
}

func TestService_SchoolTemplateVisibility(t *testing.T) {
	tests := []struct {
		name        string
		userSchool  *uint
		wantTitles  []string
		startErrFor uint // 受検開始が ErrNotFound になるテンプレート
	}{
		{"同じ学校の学生は標準と独自の両方", ptr(uint(10)), []string{"性格検査（標準）", "A校の検査"}, 0},
		{"他校の学生は標準だけ", ptr(uint(11)), []string{"性格検査（標準）"}, 2},
		{"学校未設定の学生は標準だけ", nil, []string{"性格検査（標準）"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMemRepo()
			withSchoolTemplate(repo)
			if tt.userSchool != nil {
				repo.userSchools[7] = *tt.userSchool
			}
			svc, _ := newTestService(repo, "")
			list, err := svc.List(7)
			if err != nil {
				t.Fatal(err)
			}
			var titles []string
			for _, st := range list {
				titles = append(titles, st.Title)
			}
			if strings.Join(titles, ",") != strings.Join(tt.wantTitles, ",") {
				t.Fatalf("一覧 = %v, want %v", titles, tt.wantTitles)
			}
			if tt.startErrFor != 0 {
				if _, err := svc.Start(7, tt.startErrFor); !errors.Is(err, ErrNotFound) {
					t.Fatalf("他校の独自テンプレートは受検を始められない(404): %v", err)
				}
			}
		})
	}
}

// 学校独自の提出はマッチング用の user_weight_scores へ書かず、マッチングも引き直さない。
func TestService_SchoolTemplateSubmitDoesNotMerge(t *testing.T) {
	repo := newMemRepo()
	withSchoolTemplate(repo)
	repo.userSchools[7] = 10
	svc, spy := newTestService(repo, "chat-session")
	v, err := svc.Start(7, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range v.Questions {
		if err := svc.SaveAnswer(7, v.SessionID, q.ID, 5); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Submit(context.Background(), 7, v.SessionID); err != nil {
		t.Fatal(err)
	}
	if len(repo.writtenScores) != 0 {
		t.Fatalf("学校独自はスコアを書かない: %v", repo.writtenScores)
	}
	if len(spy.calls) != 0 {
		t.Fatalf("学校独自ではマッチングを引き直さない: %v", spy.calls)
	}
	// 結果自体は本人が読める
	res, err := svc.Result(7, v.SessionID)
	if err != nil || len(res.Scores) == 0 {
		t.Fatalf("結果が読めない: %v %+v", err, res)
	}
}

func TestService_ListMarksScopeAndOverdue(t *testing.T) {
	repo := newMemRepo()
	withSchoolTemplate(repo)
	repo.userSchools[7] = 10
	past := time.Now().Add(-time.Hour)
	tpl := repo.templates[2]
	tpl.DueAt = &past
	repo.templates[2] = tpl
	svc, _ := newTestService(repo, "")
	list, _ := svc.List(7)
	byTitle := map[string]TemplateStatus{}
	for _, st := range list {
		byTitle[st.Title] = st
	}
	if got := byTitle["A校の検査"]; got.Scope != ScopeSchool || !got.Overdue {
		t.Fatalf("学校独自・期限超過の表示: %+v", got)
	}
	if got := byTitle["性格検査（標準）"]; got.Scope != ScopeStandard || got.Overdue {
		t.Fatalf("標準は期限なし: %+v", got)
	}
}

// ---- 学校側（先生） ----

type memSchoolRepo struct {
	orgs      map[uint]uint
	templates []models.AssessmentTemplate
	questions []models.AssessmentQuestion
	students  map[string][]repositories.SchoolStudent // "school:org"
	sessions  []models.AssessmentSession
	answers   map[uint]int
	results   map[uint]map[string]float64
}

func (m *memSchoolRepo) FindSchoolOrganizationID(id uint) (uint, error) {
	if o, ok := m.orgs[id]; ok {
		return o, nil
	}
	return 0, gorm.ErrRecordNotFound
}
func (m *memSchoolRepo) CreateSchoolTemplate(t *models.AssessmentTemplate, qs []models.AssessmentQuestion) error {
	t.ID = uint(len(m.templates) + 1)
	m.templates = append(m.templates, *t)
	for i := range qs {
		qs[i].TemplateID, qs[i].TemplateVersion = t.ID, t.Version
		m.questions = append(m.questions, qs[i])
	}
	return nil
}
func (m *memSchoolRepo) ListSchoolTemplates(schoolID uint) ([]models.AssessmentTemplate, error) {
	var out []models.AssessmentTemplate
	for _, t := range m.templates {
		if t.SchoolID != nil && *t.SchoolID == schoolID {
			out = append(out, t)
		}
	}
	return out, nil
}
func (m *memSchoolRepo) FindSchoolTemplate(id, schoolID uint) (*models.AssessmentTemplate, error) {
	for _, t := range m.templates {
		if t.ID == id && t.SchoolID != nil && *t.SchoolID == schoolID {
			return &t, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memSchoolRepo) ListQuestions(tid uint, v int) ([]models.AssessmentQuestion, error) {
	var out []models.AssessmentQuestion
	for _, q := range m.questions {
		if q.TemplateID == tid && q.TemplateVersion == v {
			out = append(out, q)
		}
	}
	return out, nil
}
func (m *memSchoolRepo) ListSchoolStudents(schoolID, orgID uint) ([]repositories.SchoolStudent, error) {
	return m.students[key(schoolID, orgID)], nil
}
func key(s, o uint) string { b, _ := json.Marshal([2]uint{s, o}); return string(b) }

// ListSessionsByTemplate は本物と同じ契約にする（在籍学生ごとの最新1件だけ、id 降順）。
// ここを「全件返す」ままにすると、絞り込みを DB 側へ移した変更がテストで検知できない。
func (m *memSchoolRepo) ListSessionsByTemplate(tid, schoolID, orgID uint) ([]models.AssessmentSession, error) {
	enrolled := map[uint]bool{}
	for _, stu := range m.students[key(schoolID, orgID)] {
		enrolled[stu.ID] = true
	}
	seen := map[uint]bool{}
	var out []models.AssessmentSession
	for i := len(m.sessions) - 1; i >= 0; i-- {
		ss := m.sessions[i]
		if ss.TemplateID != tid || !enrolled[ss.UserID] || seen[ss.UserID] {
			continue
		}
		seen[ss.UserID] = true
		out = append(out, ss)
	}
	return out, nil
}
func (m *memSchoolRepo) CountAnswersBySessions(ids []uint) (map[uint]int, error) {
	out := map[uint]int{}
	for _, id := range ids {
		out[id] = m.answers[id]
	}
	return out, nil
}
func (m *memSchoolRepo) ListResultsBySessions(ids []uint) (map[uint]map[string]float64, error) {
	out := map[uint]map[string]float64{}
	for _, id := range ids {
		if r, ok := m.results[id]; ok {
			out[id] = r
		}
	}
	return out, nil
}

func validInput() CreateTemplateInput {
	return CreateTemplateInput{
		Title: " A校の検査 ",
		Questions: []QuestionInput{
			{Body: "新しい技術を試すのが好きだ", WeightCategory: "技術志向"},
			{Body: "一人で進めたい", WeightCategory: "チームワーク志向", ReverseScored: true},
		},
	}
}

func TestSchoolService_CreateTemplate(t *testing.T) {
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	longBody := strings.Repeat("あ", maxQuestionBodyRunes+1)
	tests := []struct {
		name    string
		mutate  func(*CreateTemplateInput)
		wantErr string // ErrInvalidTemplate のメッセージの一部。空なら成功
	}{
		{"正常", func(*CreateTemplateInput) {}, ""},
		{"タイトル空", func(in *CreateTemplateInput) { in.Title = "  " }, "タイトル"},
		{"問題なし", func(in *CreateTemplateInput) { in.Questions = nil }, "問題は"},
		{"問題が多すぎる", func(in *CreateTemplateInput) {
			in.Questions = make([]QuestionInput, maxTemplateQuestions+1)
			for i := range in.Questions {
				in.Questions[i] = QuestionInput{Body: "x", WeightCategory: "技術志向"}
			}
		}, "問題は"},
		{"正典外カテゴリは弾く", func(in *CreateTemplateInput) { in.Questions[1].WeightCategory = "自由入力" }, "2問目のカテゴリ"},
		{"問題文が空", func(in *CreateTemplateInput) { in.Questions[0].Body = "" }, "1問目の問題文"},
		{"問題文が長すぎる", func(in *CreateTemplateInput) { in.Questions[0].Body = longBody }, "1問目の問題文"},
		{"期限が過去", func(in *CreateTemplateInput) { in.DueAt = ptr(now.Add(-time.Minute)) }, "受検期限"},
		{"期限が未来は可", func(in *CreateTemplateInput) { in.DueAt = ptr(now.Add(24 * time.Hour)) }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &memSchoolRepo{orgs: map[uint]uint{10: 3}}
			svc := NewSchoolService(repo)
			svc.now = func() time.Time { return now }
			in := validInput()
			tt.mutate(&in)
			got, err := svc.CreateTemplate(10, 99, in)
			if tt.wantErr != "" {
				var inv ErrInvalidTemplate
				if !errors.As(err, &inv) || !strings.Contains(inv.Reason, tt.wantErr) {
					t.Fatalf("err = %v, want 入力不正(%q)", err, tt.wantErr)
				}
				if len(repo.templates) != 0 {
					t.Fatal("不正な入力で保存されている")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			saved := repo.templates[0]
			// マッチングに載らないこと・テナント境界・監査用の作成者を固定する
			if saved.IsStandard || saved.SchoolID == nil || *saved.SchoolID != 10 ||
				saved.OrganizationID == nil || *saved.OrganizationID != 3 || saved.CreatedBy == nil || *saved.CreatedBy != 99 {
				t.Fatalf("保存内容: %+v", saved)
			}
			if got.Title != "A校の検査" || got.QuestionCount != 2 {
				t.Fatalf("戻り値: %+v", got)
			}
			if !repo.questions[1].ReverseScored || repo.questions[0].Position != 1 || repo.questions[1].Position != 2 {
				t.Fatalf("問題の保存: %+v", repo.questions)
			}
		})
	}
}

func TestSchoolService_CreateTemplateUnknownSchool(t *testing.T) {
	svc := NewSchoolService(&memSchoolRepo{orgs: map[uint]uint{}})
	if _, err := svc.CreateTemplate(10, 1, validInput()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("存在しない学校: %v", err)
	}
}

func statusFixture() *memSchoolRepo {
	return &memSchoolRepo{
		orgs: map[uint]uint{10: 3, 11: 3},
		templates: []models.AssessmentTemplate{
			{ID: 1, SchoolID: ptr(uint(10)), Title: "A校の検査", Version: 1, DueAt: ptr(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))},
			{ID: 2, SchoolID: ptr(uint(11)), Title: "B校の検査", Version: 1},
		},
		questions: []models.AssessmentQuestion{
			{ID: 1, TemplateID: 1, TemplateVersion: 1, Position: 1}, {ID: 2, TemplateID: 1, TemplateVersion: 1, Position: 2},
		},
		students: map[string][]repositories.SchoolStudent{
			key(10, 3): {{ID: 1, Name: "完了"}, {ID: 2, Name: "途中"}, {ID: 3, Name: "未受検"}, {ID: 4, Name: "超過で途中"}},
		},
		sessions: []models.AssessmentSession{
			{ID: 1, UserID: 1, TemplateID: 1, TemplateVersion: 1, Status: StatusSubmitted},
			{ID: 2, UserID: 2, TemplateID: 1, TemplateVersion: 1, Status: StatusInProgress},
			{ID: 3, UserID: 4, TemplateID: 1, TemplateVersion: 1, Status: StatusInProgress},
			{ID: 4, UserID: 999, TemplateID: 1, TemplateVersion: 1, Status: StatusSubmitted}, // 他校の学生の受検は出さない
		},
		answers: map[uint]int{1: 2, 2: 1, 3: 1},
		results: map[uint]map[string]float64{1: {"技術志向": 80, "チームワーク志向": 60, "創造性志向": 70, "安定志向": 10}},
	}
}

func TestSchoolService_StudentStatusesOrderAndPrivacy(t *testing.T) {
	svc := NewSchoolService(statusFixture())
	svc.now = func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) } // 期限(10/1)を過ぎている
	got, err := svc.StudentStatuses(10, 1)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, st := range got.Students {
		order = append(order, st.Name)
	}
	// 期限(10/1)を過ぎているので未提出の3人はすべて超過。同順位は元の並び(id順)を保ち、完了は最後。
	if strings.Join(order, ",") != "途中,未受検,超過で途中,完了" {
		t.Fatalf("order = %v", order)
	}
	for _, st := range got.Students[:3] {
		if !st.Overdue {
			t.Fatalf("期限を過ぎた未提出は超過: %+v", st)
		}
	}
	last := got.Students[3]
	if last.Name != "完了" || last.Overdue {
		t.Fatalf("完了は最後で、超過扱いにしない: %+v", last)
	}
	if strings.Join(last.TopCategories, ",") != "技術志向,創造性志向" {
		t.Fatalf("上位カテゴリ名のみ・スコア降順: %v", last.TopCategories)
	}
	// スコアの生値は応答に出さない
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "80") || strings.Contains(string(raw), "score") {
		t.Fatalf("スコアの生値が応答に含まれている: %s", raw)
	}
	for _, st := range got.Students {
		if st.StudentID == 999 {
			t.Fatal("他校の学生の受検が混ざっている")
		}
	}
}

func TestSchoolService_StudentStatusesRanking(t *testing.T) {
	repo := statusFixture()
	// 期限なしにすると超過は無く、未受検 → 途中 → 完了
	tpl := repo.templates[0]
	tpl.DueAt = nil
	repo.templates[0] = tpl
	svc := NewSchoolService(repo)
	got, err := svc.StudentStatuses(10, 1)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, st := range got.Students {
		order = append(order, st.Name)
	}
	if strings.Join(order, ",") != "未受検,途中,超過で途中,完了" {
		t.Fatalf("未受検 → 途中 → 完了: %v", order)
	}
	if got.Students[1].AnsweredCount != 1 || got.Students[1].QuestionCount != 2 {
		t.Fatalf("進捗: %+v", got.Students[1])
	}
}

// 他校のテンプレートの受検状況は読めない（テナント境界）。
func TestSchoolService_StudentStatusesCrossSchool(t *testing.T) {
	svc := NewSchoolService(statusFixture())
	if _, err := svc.StudentStatuses(10, 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("学校10の先生が学校11のテンプレートを読めてはいけない: %v", err)
	}
	if _, err := svc.StudentStatuses(11, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("逆方向も同じ: %v", err)
	}
}

func TestCategories_AreCanonicalTen(t *testing.T) {
	got := Categories()
	if len(got) != 10 || got[0] != "技術志向" || got[9] != "コミュニケーション力" {
		t.Fatalf("正典10カテゴリ: %v", got)
	}
}

// 結果画面が使われ方を出し分けられるよう、結果に提供元を含める。
func TestService_ResultCarriesScope(t *testing.T) {
	repo := newMemRepo()
	withSchoolTemplate(repo)
	repo.userSchools[7] = 10
	svc, _ := newTestService(repo, "")
	for _, tc := range []struct {
		templateID uint
		want       string
	}{{1, ScopeStandard}, {2, ScopeSchool}} {
		v, err := svc.Start(7, tc.templateID)
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range v.Questions {
			if err := svc.SaveAnswer(7, v.SessionID, q.ID, 4); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := svc.Submit(context.Background(), 7, v.SessionID); err != nil {
			t.Fatal(err)
		}
		res, err := svc.Result(7, v.SessionID)
		if err != nil || res.Scope != tc.want {
			t.Fatalf("template %d scope = %q (%v), want %q", tc.templateID, res.Scope, err, tc.want)
		}
	}
}
