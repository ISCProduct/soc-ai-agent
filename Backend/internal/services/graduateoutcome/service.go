// Package graduateoutcome は学生向けに、自分の学校の卒業生の進路実績を集計して返す（#2032）。
//
// 先輩の進路は学校の強みだが、学生が見る導線が無かった。特定企業の推薦ではなく事実の提示なので、
// 並びは人数の多い順（同数は企業名順）にし、優遇の印は付けない。卒業生の氏名などは出さず人数だけ。
package graduateoutcome

import (
	"sort"

	"Backend/internal/repositories"
)

// Store は進路実績の集計（repositories.GraduateOutcomeRepository が満たす）。
type Store interface {
	StudentSchoolID(userID uint) (*uint, error)
	CountsByCompanyYear(schoolID uint, f repositories.GraduateOutcomeFilter) ([]repositories.GraduateOutcomeRow, error)
	FilterOptions(schoolID uint) ([]int, []string, error)
	CompanyCounts(schoolID, companyID uint) ([]repositories.GraduateOutcomeRow, []repositories.GraduatePositionCount, error)
}

type YearCount struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

type CompanyOutcome struct {
	CompanyID   uint        `json:"company_id"`
	CompanyName string      `json:"company_name"`
	Industry    string      `json:"industry"`
	Total       int         `json:"total"`
	ByYear      []YearCount `json:"by_year"`
}

// List は一覧の結果。SchoolLinked が false なら学校が未登録で、実績は出さない（他校の実績は見せない）。
type List struct {
	SchoolLinked bool             `json:"school_linked"`
	Total        int              `json:"total"`
	Companies    []CompanyOutcome `json:"companies"`
	Years        []int            `json:"years"`
	Industries   []string         `json:"industries"`
}

type PositionCount struct {
	Title string `json:"title"`
	Count int    `json:"count"`
}

// CompanyDetail は1社の実績。
type CompanyDetail struct {
	SchoolLinked bool            `json:"school_linked"`
	Total        int             `json:"total"`
	ByYear       []YearCount     `json:"by_year"`
	ByPosition   []PositionCount `json:"by_position"`
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

// List は学生の学校の進路実績を、企業ごとにまとめて返す。
func (s *Service) List(userID uint, f repositories.GraduateOutcomeFilter) (*List, error) {
	out := &List{Companies: []CompanyOutcome{}, Years: []int{}, Industries: []string{}}
	schoolID, err := s.store.StudentSchoolID(userID)
	if err != nil || schoolID == nil {
		return out, err
	}
	out.SchoolLinked = true
	rows, err := s.store.CountsByCompanyYear(*schoolID, f)
	if err != nil {
		return nil, err
	}
	byCompany := map[uint]*CompanyOutcome{}
	for _, r := range rows {
		c, ok := byCompany[r.CompanyID]
		if !ok {
			c = &CompanyOutcome{CompanyID: r.CompanyID, CompanyName: r.CompanyName, Industry: r.Industry, ByYear: []YearCount{}}
			byCompany[r.CompanyID] = c
		}
		c.Total += r.Count
		c.ByYear = append(c.ByYear, YearCount{Year: r.GraduationYear, Count: r.Count})
		out.Total += r.Count
	}
	for _, c := range byCompany {
		sort.Slice(c.ByYear, func(i, j int) bool { return c.ByYear[i].Year > c.ByYear[j].Year })
		out.Companies = append(out.Companies, *c)
	}
	sort.Slice(out.Companies, func(i, j int) bool {
		if out.Companies[i].Total != out.Companies[j].Total {
			return out.Companies[i].Total > out.Companies[j].Total
		}
		return out.Companies[i].CompanyName < out.Companies[j].CompanyName
	})
	years, industries, err := s.store.FilterOptions(*schoolID)
	if err != nil {
		return nil, err
	}
	if years != nil {
		out.Years = years
	}
	if industries != nil {
		out.Industries = industries
	}
	return out, nil
}

// Company は学生の学校の、1社への進路実績を返す。
func (s *Service) Company(userID, companyID uint) (*CompanyDetail, error) {
	out := &CompanyDetail{ByYear: []YearCount{}, ByPosition: []PositionCount{}}
	schoolID, err := s.store.StudentSchoolID(userID)
	if err != nil || schoolID == nil {
		return out, err
	}
	out.SchoolLinked = true
	byYear, byPosition, err := s.store.CompanyCounts(*schoolID, companyID)
	if err != nil {
		return nil, err
	}
	for _, r := range byYear {
		out.ByYear = append(out.ByYear, YearCount{Year: r.GraduationYear, Count: r.Count})
		out.Total += r.Count
	}
	for _, p := range byPosition {
		title := p.Title
		if title == "" {
			title = "職種の登録なし"
		}
		out.ByPosition = append(out.ByPosition, PositionCount{Title: title, Count: p.Count})
	}
	return out, nil
}
