package assessment

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"Backend/domain/valueobject"
	"Backend/internal/models"
	"Backend/internal/repositories"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 学校独自テンプレートの入力上限。問題数が多すぎると学生が離脱するため上限を置く。
const (
	maxTemplateTitleRunes = 100
	maxTemplateDescRunes  = 500
	maxQuestionBodyRunes  = 200
	minTemplateQuestions  = 1
	maxTemplateQuestions  = 60
	topCategoryCount      = 2
)

// ErrInvalidTemplate は学校独自テンプレートの入力不正。メッセージは先生にそのまま見せてよい。
type ErrInvalidTemplate struct{ Reason string }

func (e ErrInvalidTemplate) Error() string { return e.Reason }

// SchoolRepository は学校側（先生）の操作が使う永続化の面。
// どの操作も schoolID を必須にして、他校のテンプレート・学生に触れない形にする（#1675）。
type SchoolRepository interface {
	FindSchoolOrganizationID(schoolID uint) (uint, error)
	CreateSchoolTemplate(t *models.AssessmentTemplate, qs []models.AssessmentQuestion) error
	ListSchoolTemplates(schoolID uint) ([]models.AssessmentTemplate, error)
	FindSchoolTemplate(id, schoolID uint) (*models.AssessmentTemplate, error)
	ListQuestions(templateID uint, version int) ([]models.AssessmentQuestion, error)
	ListSchoolStudents(schoolID, organizationID uint) ([]repositories.SchoolStudent, error)
	ListSessionsByTemplate(templateID uint) ([]models.AssessmentSession, error)
	CountAnswersBySessions(sessionIDs []uint) (map[uint]int, error)
	ListResultsBySessions(sessionIDs []uint) (map[uint]map[string]float64, error)
}

// SchoolService は学校独自テンプレートの作成と、先生向けの受検状況一覧。
// 学校独自の結果はマッチングに使わない。ここは参考値を学校内で見せるだけ。
type SchoolService struct {
	repo SchoolRepository
	now  func() time.Time
}

func NewSchoolService(repo SchoolRepository) *SchoolService {
	return &SchoolService{repo: repo, now: time.Now}
}

type QuestionInput struct {
	Body           string `json:"body"`
	WeightCategory string `json:"weight_category"`
	ReverseScored  bool   `json:"reverse_scored"`
}

type CreateTemplateInput struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	DueAt       *time.Time      `json:"due_at"`
	Questions   []QuestionInput `json:"questions"`
}

type SchoolTemplateSummary struct {
	TemplateID    uint       `json:"template_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	QuestionCount int        `json:"question_count"`
	DueAt         *time.Time `json:"due_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

func validateTemplateInput(in *CreateTemplateInput, now time.Time) error {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	if in.Title == "" {
		return ErrInvalidTemplate{"タイトルを入力してください"}
	}
	if utf8.RuneCountInString(in.Title) > maxTemplateTitleRunes {
		return ErrInvalidTemplate{fmt.Sprintf("タイトルは%d文字以内にしてください", maxTemplateTitleRunes)}
	}
	if utf8.RuneCountInString(in.Description) > maxTemplateDescRunes {
		return ErrInvalidTemplate{fmt.Sprintf("説明は%d文字以内にしてください", maxTemplateDescRunes)}
	}
	if in.DueAt != nil && !in.DueAt.After(now) {
		return ErrInvalidTemplate{"受検期限は現在より後の日時にしてください"}
	}
	if n := len(in.Questions); n < minTemplateQuestions || n > maxTemplateQuestions {
		return ErrInvalidTemplate{fmt.Sprintf("問題は%d〜%d問で作成してください", minTemplateQuestions, maxTemplateQuestions)}
	}
	for i := range in.Questions {
		q := &in.Questions[i]
		q.Body = strings.TrimSpace(q.Body)
		if q.Body == "" {
			return ErrInvalidTemplate{fmt.Sprintf("%d問目の問題文を入力してください", i+1)}
		}
		if utf8.RuneCountInString(q.Body) > maxQuestionBodyRunes {
			return ErrInvalidTemplate{fmt.Sprintf("%d問目の問題文は%d文字以内にしてください", i+1, maxQuestionBodyRunes)}
		}
		// 正典10カテゴリ以外は受け付けない（自由入力させない）。
		cat, err := valueobject.ParseWeightCategory(q.WeightCategory)
		if err != nil {
			return ErrInvalidTemplate{fmt.Sprintf("%d問目のカテゴリが不正です", i+1)}
		}
		q.WeightCategory = string(cat)
	}
	return nil
}

// Categories は問題に付けられる正典カテゴリ。作成画面の選択肢の正本（FE に複製を持たせない）。
func Categories() []string {
	all := valueobject.AllWeightCategories()
	out := make([]string, len(all))
	for i, c := range all {
		out[i] = string(c)
	}
	return out
}

// CreateTemplate は学校独自テンプレートを作る。is_standard は常に false。
func (s *SchoolService) CreateTemplate(schoolID, createdBy uint, in CreateTemplateInput) (*SchoolTemplateSummary, error) {
	if err := validateTemplateInput(&in, s.now()); err != nil {
		return nil, err
	}
	orgID, err := s.repo.FindSchoolOrganizationID(schoolID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t := &models.AssessmentTemplate{
		SchoolID: &schoolID, OrganizationID: &orgID,
		Slug:  "school-" + fmt.Sprint(schoolID) + "-" + uuid.NewString()[:8],
		Title: in.Title, Description: in.Description, Version: 1,
		IsStandard: false, DueAt: in.DueAt, CreatedBy: &createdBy,
	}
	qs := make([]models.AssessmentQuestion, len(in.Questions))
	for i, q := range in.Questions {
		qs[i] = models.AssessmentQuestion{Position: i + 1, Body: q.Body, WeightCategory: q.WeightCategory, ReverseScored: q.ReverseScored}
	}
	if err := s.repo.CreateSchoolTemplate(t, qs); err != nil {
		return nil, err
	}
	return &SchoolTemplateSummary{TemplateID: t.ID, Title: t.Title, Description: t.Description,
		QuestionCount: len(qs), DueAt: t.DueAt, CreatedAt: t.CreatedAt}, nil
}

// ListTemplates は学校独自テンプレートの一覧（運営標準は含まない）。
func (s *SchoolService) ListTemplates(schoolID uint) ([]SchoolTemplateSummary, error) {
	ts, err := s.repo.ListSchoolTemplates(schoolID)
	if err != nil {
		return nil, err
	}
	out := make([]SchoolTemplateSummary, 0, len(ts))
	for _, t := range ts {
		qs, err := s.repo.ListQuestions(t.ID, t.Version)
		if err != nil {
			return nil, err
		}
		out = append(out, SchoolTemplateSummary{TemplateID: t.ID, Title: t.Title, Description: t.Description,
			QuestionCount: len(qs), DueAt: t.DueAt, CreatedAt: t.CreatedAt})
	}
	return out, nil
}

// StudentStatus は先生向け一覧の1行。スコアの生値は持たない。
// 数字を並べると成績表として読まれ、学生が正直に答えなくなるため、上位カテゴリ名だけ返す。
type StudentStatus struct {
	StudentID     uint       `json:"student_id"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	Overdue       bool       `json:"overdue"`
	AnsweredCount int        `json:"answered_count"`
	QuestionCount int        `json:"question_count"`
	SubmittedAt   *time.Time `json:"submitted_at,omitempty"`
	TopCategories []string   `json:"top_categories"`
}

type StudentStatusList struct {
	TemplateID uint            `json:"template_id"`
	Title      string          `json:"title"`
	DueAt      *time.Time      `json:"due_at,omitempty"`
	Students   []StudentStatus `json:"students"`
}

// 並び順: 期限超過 → 未受検 → 途中 → 完了。「誰を支援すべきか」が最上部に来る。
func statusRank(st StudentStatus) int {
	switch {
	case st.Overdue:
		return 0
	case st.Status == StatusNotStarted:
		return 1
	case st.Status == StatusInProgress:
		return 2
	default:
		return 3
	}
}

// StudentStatuses は指定校の独自テンプレートの受検状況。他校のテンプレートは ErrNotFound。
func (s *SchoolService) StudentStatuses(schoolID, templateID uint) (*StudentStatusList, error) {
	t, err := s.repo.FindSchoolTemplate(templateID, schoolID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	orgID, err := s.repo.FindSchoolOrganizationID(schoolID)
	if err != nil {
		return nil, err
	}
	students, err := s.repo.ListSchoolStudents(schoolID, orgID)
	if err != nil {
		return nil, err
	}
	sessions, err := s.repo.ListSessionsByTemplate(t.ID)
	if err != nil {
		return nil, err
	}
	// 学生ごとの最新の受検（ListSessionsByTemplate は id 降順）。学校の在籍学生だけ使う。
	latest := map[uint]models.AssessmentSession{}
	var sessionIDs []uint
	for _, ss := range sessions {
		if _, ok := latest[ss.UserID]; !ok {
			latest[ss.UserID] = ss
			sessionIDs = append(sessionIDs, ss.ID)
		}
	}
	counts, err := s.repo.CountAnswersBySessions(sessionIDs)
	if err != nil {
		return nil, err
	}
	results, err := s.repo.ListResultsBySessions(sessionIDs)
	if err != nil {
		return nil, err
	}
	// 受検中の版で数える。版は学生ごとに凍結されているため、最新の受検の版を使う。
	qcache := map[int]int{}
	questionCount := func(version int) (int, error) {
		if n, ok := qcache[version]; ok {
			return n, nil
		}
		qs, err := s.repo.ListQuestions(t.ID, version)
		qcache[version] = len(qs)
		return len(qs), err
	}
	total, err := questionCount(t.Version)
	if err != nil {
		return nil, err
	}

	now := s.now()
	rows := make([]StudentStatus, 0, len(students))
	for _, stu := range students {
		row := StudentStatus{StudentID: stu.ID, Name: stu.Name, Status: StatusNotStarted, QuestionCount: total, TopCategories: []string{}}
		if ss, ok := latest[stu.ID]; ok {
			row.Status = ss.Status
			row.AnsweredCount = counts[ss.ID]
			row.SubmittedAt = ss.SubmittedAt
			if row.QuestionCount, err = questionCount(ss.TemplateVersion); err != nil {
				return nil, err
			}
			if ss.Status == StatusSubmitted {
				row.TopCategories = topCategories(results[ss.ID], topCategoryCount)
			}
		}
		row.Overdue = isOverdue(t.DueAt, row.Status, now)
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool { return statusRank(rows[i]) < statusRank(rows[j]) })
	return &StudentStatusList{TemplateID: t.ID, Title: t.Title, DueAt: t.DueAt, Students: rows}, nil
}

// topCategories はスコアの高い順に n 個のカテゴリ名。同点は正典の並びを優先する（結果が実行ごとに揺れない）。
func topCategories(scores map[string]float64, n int) []string {
	out := []string{}
	for _, c := range valueobject.AllWeightCategories() {
		if _, ok := scores[string(c)]; ok {
			out = append(out, string(c))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return scores[out[i]] > scores[out[j]] })
	if len(out) > n {
		out = out[:n]
	}
	return out
}
