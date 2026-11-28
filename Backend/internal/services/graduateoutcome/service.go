// Package graduateoutcome は学生向けに、自分の学校の卒業生の進路実績を集計して返す（#2032）。
//
// 先輩の進路は学校の強みだが、学生が見る導線が無かった。特定企業の推薦ではなく事実の提示なので、
// 並びは人数の多い順（同数は企業名順）にし、優遇の印は付けない。卒業生の氏名などは出さず人数だけ。
//
// 人数が MinCount 未満の区分は数を出さず「少数」にまとめる。母数の小さい学校では、
// 「企業 × 年度」や「企業 × 職種」で 1 人と分かると、学内では誰のことか特定できるため。
//
// 秘匿は最小の区分（企業 × 年度 × 職種）で行い、企業の合計・年度別・職種別・一覧全体の合計は、
// どれも「見せる最小の区分の和」だけで作る。隠した人数はどの数字にも入らないので、合計から内訳を
// 引いても、年度別と職種別を比べても、年度で絞り込む前後を比べても、隠した人数は求まらない。
// 以前は正確な合計を出したまま内訳だけを隠していたので、「合計 − 表示中の内訳」で分かった。
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
	// OutcomeCells は最小の区分（企業 × 年度 × 職種）ごとの人数を返す
	OutcomeCells(ctx context.Context, schoolID uint, f repositories.GraduateOutcomeFilter) ([]repositories.GraduateOutcomeRow, error)
	FilterOptions(ctx context.Context, schoolID uint) ([]int, []string, error)
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

// CompanyOutcome は1社の実績。Total・ByYear は見せる最小の区分の和。
// Total が nil なら、見せる区分が無い（すべて少数）。TotalFew・ByYearFew は、数に入れていない
// 少数の卒業生がほかにいることを表す（画面では「＋少数」）。
type CompanyOutcome struct {
	CompanyID   uint        `json:"company_id"`
	CompanyName string      `json:"company_name"`
	Industry    string      `json:"industry"`
	Total       *int        `json:"total"`
	TotalFew    bool        `json:"total_few"`
	ByYear      []YearCount `json:"by_year"`
	ByYearFew   bool        `json:"by_year_few"`
}

// List は一覧の結果。SchoolLinked が false なら学校が未登録で、実績は出さない（他校の実績は見せない）。
// Total は見せる区分の和で、TotalFew は数に入れていない少数の卒業生がほかにいることを表す。
type List struct {
	SchoolLinked bool             `json:"school_linked"`
	Total        int              `json:"total"`
	TotalFew     bool             `json:"total_few"`
	Companies    []CompanyOutcome `json:"companies"`
	Years        []int            `json:"years"`
	Industries   []string         `json:"industries"`
}

type PositionCount struct {
	Title string `json:"title"`
	Count int    `json:"count"`
}

// CompanyDetail は1社の実績。Total・ByYear・ByPosition はどれも見せる最小の区分の和なので、
// 年度別の和も職種別の和も Total と一致する。Total が nil なら見せる区分が無い（すべて少数）。
// TotalFew・ByYearFew・ByPositionFew は、数に入れていない少数の卒業生がほかにいることを表す。
type CompanyDetail struct {
	SchoolLinked  bool            `json:"school_linked"`
	Total         *int            `json:"total"`
	TotalFew      bool            `json:"total_few"`
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

// visibleCells は最小の区分のうち MinCount 以上のもの（見せるもの）と、MinCount 未満の区分があったかを返す。
func visibleCells(rows []repositories.GraduateOutcomeRow) (shown []repositories.GraduateOutcomeRow, hidden bool) {
	for _, r := range rows {
		if r.Count < MinCount {
			hidden = true
			continue
		}
		shown = append(shown, r)
	}
	return shown, hidden
}

// List は学生の学校の進路実績を、企業ごとにまとめて返す。数はどれも見せる最小の区分の和。
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
	rows, err := s.store.OutcomeCells(ctx, *schoolID, repositories.GraduateOutcomeFilter{Year: f.Year, Industry: f.Industry})
	if err != nil {
		return nil, err
	}
	type agg struct {
		c      CompanyOutcome
		total  int
		years  map[int]int
		hidden bool
	}
	byCompany := map[uint]*agg{}
	var order []uint
	for _, r := range rows {
		a, ok := byCompany[r.CompanyID]
		if !ok {
			a = &agg{c: CompanyOutcome{CompanyID: r.CompanyID, CompanyName: r.CompanyName, Industry: r.Industry, ByYear: []YearCount{}}, years: map[int]int{}}
			byCompany[r.CompanyID] = a
			order = append(order, r.CompanyID)
		}
		if r.Count < MinCount {
			a.hidden = true
			continue
		}
		a.total += r.Count
		a.years[r.GraduationYear] += r.Count
	}
	for _, id := range order {
		a := byCompany[id]
		a.c.TotalFew, a.c.ByYearFew = a.hidden, a.hidden
		out.TotalFew = out.TotalFew || a.hidden
		if a.total > 0 {
			t := a.total
			a.c.Total = &t
			a.c.ByYear = sortedYears(a.years)
			out.Total += a.total
		}
		out.Companies = append(out.Companies, a.c)
	}
	// 人数の多い順。見せる区分の無い企業は末尾に企業名順で置く（並びから人数を推し量らせない）。
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
// 合計・年度別・職種別は、どれも同じ「見せる最小の区分」の和で作る。
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
	rows, err := s.store.OutcomeCells(ctx, *schoolID, repositories.GraduateOutcomeFilter{CompanyID: companyID})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		zero := 0
		out.Total = &zero
		return out, nil
	}
	shown, hidden := visibleCells(rows)
	out.TotalFew, out.ByYearFew, out.ByPositionFew = hidden, hidden, hidden
	if len(shown) == 0 {
		// 見せる区分が無い。人数も内訳も出さない
		return out, nil
	}
	total := 0
	years := map[int]int{}
	positions := map[string]int{}
	for _, r := range shown {
		total += r.Count
		years[r.GraduationYear] += r.Count
		positions[r.Title] += r.Count
	}
	out.Total = &total
	out.ByYear = sortedYears(years)
	for title, n := range positions {
		if title == "" {
			title = "職種の登録なし"
		}
		out.ByPosition = append(out.ByPosition, PositionCount{Title: title, Count: n})
	}
	// 人数の多い順、同数は職種名順
	sort.Slice(out.ByPosition, func(i, j int) bool {
		a, b := out.ByPosition[i], out.ByPosition[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Title < b.Title
	})
	return out, nil
}

// sortedYears は年度ごとの人数を、新しい順（年度未登録は最後）に並べる。
func sortedYears(years map[int]int) []YearCount {
	out := make([]YearCount, 0, len(years))
	for y, n := range years {
		out = append(out, YearCount{Year: y, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Year, out[j].Year
		if (a == 0) != (b == 0) {
			return a != 0
		}
		return a > b
	})
	return out
}
