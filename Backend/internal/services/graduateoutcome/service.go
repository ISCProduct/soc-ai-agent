// Package graduateoutcome は学生向けに、自分の学校の卒業生の進路実績を集計して返す（#2032）。
//
// 先輩の進路は学校の強みだが、学生が見る導線が無かった。特定企業の推薦ではなく事実の提示なので、
// 並びは人数の多い順（同数は企業名順）にし、優遇の印は付けない。卒業生の氏名などは出さず人数だけ。
//
// 人数が MinCount 未満の区分は数を出さず「少数」にまとめる。母数の小さい学校では、
// 「企業 × 年度」や「企業 × 職種」で 1 人と分かると、学内では誰のことか特定できるため。
package graduateoutcome

import (
	"context"
	"errors"
	"sort"

	"Backend/internal/repositories"
)

// MinCount はこの人数未満の区分を「少数」にまとめる閾値。
const MinCount = 3

var (
	// ErrUserNotFound は学生が存在しない（退会済みを含む）。
	ErrUserNotFound = errors.New("graduate outcome: user not found")
	// ErrCompanyNotFound は企業が存在しない、または学生に見せない企業。
	ErrCompanyNotFound = errors.New("graduate outcome: company not found")
)

// Store は進路実績の集計（repositories.GraduateOutcomeRepository が満たす）。
type Store interface {
	StudentSchoolID(ctx context.Context, userID uint) (*uint, error)
	CompanyVisible(ctx context.Context, companyID uint) (bool, error)
	CountsByCompanyYear(ctx context.Context, schoolID uint, f repositories.GraduateOutcomeFilter) ([]repositories.GraduateOutcomeRow, error)
	FilterOptions(ctx context.Context, schoolID uint) ([]int, []string, error)
	CompanyCounts(ctx context.Context, schoolID, companyID uint) ([]repositories.GraduateOutcomeRow, []repositories.GraduatePositionCount, error)
}

// Filter は一覧の絞り込み。0・空は絞らない。
type Filter struct {
	Year     int
	Industry string
}

// YearCount は年度ごとの人数。Year が 0 なら年度が未登録。
type YearCount struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

// CompanyOutcome は1社の実績。Total が nil なら MinCount 未満（少数）で、年度別も出さない。
// ByYearFew は、ByYear に出していない（MinCount 未満の）年度がほかにあることを表す。
type CompanyOutcome struct {
	CompanyID   uint        `json:"company_id"`
	CompanyName string      `json:"company_name"`
	Industry    string      `json:"industry"`
	Total       *int        `json:"total"`
	ByYear      []YearCount `json:"by_year"`
	ByYearFew   bool        `json:"by_year_few"`
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

// CompanyDetail は1社の実績。Total が nil なら MinCount 未満（少数）で、内訳も出さない。
// ByYearFew・ByPositionFew は、出していない（MinCount 未満の）年度・職種がほかにあることを表す。
type CompanyDetail struct {
	SchoolLinked  bool            `json:"school_linked"`
	Total         *int            `json:"total"`
	ByYear        []YearCount     `json:"by_year"`
	ByYearFew     bool            `json:"by_year_few"`
	ByPosition    []PositionCount `json:"by_position"`
	ByPositionFew bool            `json:"by_position_few"`
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) schoolID(ctx context.Context, userID uint) (*uint, error) {
	id, err := s.store.StudentSchoolID(ctx, userID)
	if errors.Is(err, repositories.ErrGraduateOutcomeUserNotFound) {
		return nil, ErrUserNotFound
	}
	return id, err
}

// List は学生の学校の進路実績を、企業ごとにまとめて返す。
func (s *Service) List(ctx context.Context, userID uint, f Filter) (*List, error) {
	schoolID, err := s.schoolID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := &List{Companies: []CompanyOutcome{}, Years: []int{}, Industries: []string{}}
	if schoolID == nil {
		return out, nil
	}
	out.SchoolLinked = true
	rows, err := s.store.CountsByCompanyYear(ctx, *schoolID, repositories.GraduateOutcomeFilter{Year: f.Year, Industry: f.Industry})
	if err != nil {
		return nil, err
	}
	type agg struct {
		c     CompanyOutcome
		total int
		years []YearCount
	}
	byCompany := map[uint]*agg{}
	for _, r := range rows {
		a, ok := byCompany[r.CompanyID]
		if !ok {
			a = &agg{c: CompanyOutcome{CompanyID: r.CompanyID, CompanyName: r.CompanyName, Industry: r.Industry, ByYear: []YearCount{}}}
			byCompany[r.CompanyID] = a
		}
		a.total += r.Count
		a.years = append(a.years, YearCount{Year: r.GraduationYear, Count: r.Count})
		out.Total += r.Count
	}
	totals := map[uint]int{}
	for id, a := range byCompany {
		totals[id] = a.total
		if a.total >= MinCount {
			t := a.total
			a.c.Total = &t
			a.c.ByYear, a.c.ByYearFew = splitYears(a.years)
		}
		out.Companies = append(out.Companies, a.c)
	}
	// 人数の多い順。少数の企業は人数（1 か 2 か）が並びから分からないよう、末尾に企業名順で置く。
	sort.Slice(out.Companies, func(i, j int) bool {
		a, b := out.Companies[i], out.Companies[j]
		if (a.Total == nil) != (b.Total == nil) {
			return a.Total != nil
		}
		if a.Total != nil && *a.Total != *b.Total {
			return *a.Total > *b.Total
		}
		if a.CompanyName != b.CompanyName {
			return a.CompanyName < b.CompanyName
		}
		return a.CompanyID < b.CompanyID
	})
	years, industries, err := s.store.FilterOptions(ctx, *schoolID)
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

// Company は学生の学校の、1社への進路実績を返す。企業が無い（見せない）なら ErrCompanyNotFound。
func (s *Service) Company(ctx context.Context, userID, companyID uint) (*CompanyDetail, error) {
	schoolID, err := s.schoolID(ctx, userID)
	if err != nil {
		return nil, err
	}
	visible, err := s.store.CompanyVisible(ctx, companyID)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, ErrCompanyNotFound
	}
	out := &CompanyDetail{ByYear: []YearCount{}, ByPosition: []PositionCount{}}
	if schoolID == nil {
		return out, nil
	}
	out.SchoolLinked = true
	byYear, byPosition, err := s.store.CompanyCounts(ctx, *schoolID, companyID)
	if err != nil {
		return nil, err
	}
	total := 0
	years := make([]YearCount, 0, len(byYear))
	for _, r := range byYear {
		years = append(years, YearCount{Year: r.GraduationYear, Count: r.Count})
		total += r.Count
	}
	if total == 0 {
		zero := 0
		out.Total = &zero
		return out, nil
	}
	if total < MinCount {
		// 少数。人数も内訳も出さない
		return out, nil
	}
	out.Total = &total
	out.ByYear, out.ByYearFew = splitYears(years)
	for _, p := range byPosition {
		if p.Count < MinCount {
			out.ByPositionFew = true
			continue
		}
		title := p.Title
		if title == "" {
			title = "職種の登録なし"
		}
		out.ByPosition = append(out.ByPosition, PositionCount{Title: title, Count: p.Count})
	}
	return out, nil
}

// splitYears は MinCount 以上の年度だけを新しい順（年度未登録は最後）で返し、MinCount 未満の年度があれば few を立てる。
func splitYears(years []YearCount) (shown []YearCount, few bool) {
	shown = []YearCount{}
	for _, y := range years {
		if y.Count < MinCount {
			few = true
			continue
		}
		shown = append(shown, y)
	}
	sort.Slice(shown, func(i, j int) bool {
		a, b := shown[i].Year, shown[j].Year
		if (a == 0) != (b == 0) {
			return a != 0
		}
		return a > b
	})
	return shown, few
}
