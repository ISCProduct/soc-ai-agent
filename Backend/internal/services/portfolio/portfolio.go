// Package portfolio は学生の自己分析ポートフォリオ（診断＋面接＋ES＋適性検査）を1つにまとめる（#2030）。
//
// 就活の提出や振り返りに使える総合のアウトプット。画面で内容を選び、ブラウザの印刷で PDF にする。
// 学生の個人情報なので本人だけが見られる（宛先は認証から決め、共有リンクは作らない）。
package portfolio

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"Backend/domain/entity"
	"Backend/internal/repositories"
)

// 出力する内容（セクション）。
const (
	SectionDiagnosis  = "diagnosis"  // 適性の診断（10カテゴリのスコア）
	SectionInterview  = "interview"  // AI 面接のレポート
	SectionES         = "es"         // ES 添削
	SectionAssessment = "assessment" // 学校の適性検査
)

// AllSections は既定で出す内容。
var AllSections = []string{SectionDiagnosis, SectionInterview, SectionES, SectionAssessment}

const (
	interviewLimit  = 3
	esLimit         = 5
	assessmentLimit = 3
)

// ParseSections はカンマ区切りの指定を、知っているセクションだけに絞って返す。空なら全部。
func ParseSections(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return AllSections
	}
	known := map[string]bool{}
	for _, s := range AllSections {
		known[s] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		s := strings.TrimSpace(part)
		if known[s] && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// Store はポートフォリオの材料（本人の分）を返す。
type Store interface {
	RecentInterviews(userID uint, limit int) ([]repositories.PortfolioInterview, error)
	RecentESReviews(userID uint, limit int) ([]repositories.PortfolioES, error)
	RecentAssessments(userID uint, limit int) ([]repositories.PortfolioAssessment, error)
}

// LatestScores は最新の診断のスコアを返す（repositories.UserWeightScoreRepository が満たす）。
type LatestScores interface {
	FindLatestByUser(userID uint) ([]entity.UserWeightScore, error)
}

type CategoryScore struct {
	Category string `json:"category"`
	Score    int    `json:"score"`
}

type Diagnosis struct {
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
	Scores    []CategoryScore `json:"scores"`
}

type Interview struct {
	SessionID    uint           `json:"session_id"`
	CreatedAt    time.Time      `json:"created_at"`
	SummaryText  string         `json:"summary_text"`
	Scores       map[string]int `json:"scores"`
	Strengths    []string       `json:"strengths"`
	Improvements []string       `json:"improvements"`
}

type Assessment struct {
	CreatedAt time.Time       `json:"created_at"`
	Title     string          `json:"title"`
	Scores    []CategoryScore `json:"scores"`
}

// Portfolio はまとめた結果。選ばなかったセクションは nil（JSON では省く）。
type Portfolio struct {
	GeneratedAt time.Time                  `json:"generated_at"`
	Sections    []string                   `json:"sections"`
	Diagnosis   *Diagnosis                 `json:"diagnosis,omitempty"`
	Interviews  []Interview                `json:"interviews,omitempty"`
	ESReviews   []repositories.PortfolioES `json:"es_reviews,omitempty"`
	Assessments []Assessment               `json:"assessments,omitempty"`
}

type Service struct {
	store  Store
	scores LatestScores
	now    func() time.Time
}

func NewService(store Store, scores LatestScores) *Service {
	return &Service{store: store, scores: scores, now: time.Now}
}

// Build は本人のポートフォリオを、選んだセクションだけで組み立てる。
func (s *Service) Build(userID uint, sections []string) (*Portfolio, error) {
	p := &Portfolio{GeneratedAt: s.now(), Sections: sections}
	for _, section := range sections {
		switch section {
		case SectionDiagnosis:
			rows, err := s.scores.FindLatestByUser(userID)
			if err != nil {
				return nil, err
			}
			d := &Diagnosis{Scores: []CategoryScore{}}
			for _, r := range rows {
				d.Scores = append(d.Scores, CategoryScore{Category: r.WeightCategory, Score: r.Score})
				if d.UpdatedAt == nil || r.UpdatedAt.After(*d.UpdatedAt) {
					t := r.UpdatedAt
					d.UpdatedAt = &t
				}
			}
			sortScores(d.Scores)
			p.Diagnosis = d
		case SectionInterview:
			rows, err := s.store.RecentInterviews(userID, interviewLimit)
			if err != nil {
				return nil, err
			}
			p.Interviews = make([]Interview, 0, len(rows))
			for _, r := range rows {
				p.Interviews = append(p.Interviews, Interview{
					SessionID:    r.SessionID,
					CreatedAt:    r.CreatedAt,
					SummaryText:  r.SummaryText,
					Scores:       parseIntMap(r.ScoresJSON),
					Strengths:    parseStrings(r.StrengthsJSON),
					Improvements: parseStrings(r.ImprovementsJSON),
				})
			}
		case SectionES:
			rows, err := s.store.RecentESReviews(userID, esLimit)
			if err != nil {
				return nil, err
			}
			p.ESReviews = rows
			if p.ESReviews == nil {
				p.ESReviews = []repositories.PortfolioES{}
			}
		case SectionAssessment:
			rows, err := s.store.RecentAssessments(userID, assessmentLimit)
			if err != nil {
				return nil, err
			}
			p.Assessments = make([]Assessment, 0, len(rows))
			for _, r := range rows {
				scores := []CategoryScore{}
				for k, v := range parseIntMap(r.CategoryScoresJSON) {
					scores = append(scores, CategoryScore{Category: k, Score: v})
				}
				sortScores(scores)
				p.Assessments = append(p.Assessments, Assessment{CreatedAt: r.CreatedAt, Title: r.Title, Scores: scores})
			}
		}
	}
	return p, nil
}

// sortScores は高い順（同点はカテゴリ名順）に並べる。強みが先に来るように。
func sortScores(scores []CategoryScore) {
	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].Score != scores[j].Score {
			return scores[i].Score > scores[j].Score
		}
		return scores[i].Category < scores[j].Category
	})
}

// parseIntMap は {"名前": 数値} の JSON を読む。壊れていれば空（ポートフォリオ全体は止めない）。
func parseIntMap(raw string) map[string]int {
	out := map[string]int{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	var m map[string]float64
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return out
	}
	for k, v := range m {
		out[k] = int(v + 0.5)
	}
	return out
}

// parseStrings は文字列の配列の JSON を読む。壊れていれば空。
func parseStrings(raw string) []string {
	out := []string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		out = []string{}
	}
	return out
}
