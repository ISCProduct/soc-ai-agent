package graduateoutcome

import (
	"errors"
	"testing"

	"Backend/internal/repositories"
)

// #2032: 学生向けの卒業生の進路実績。

type fakeStore struct {
	school     *uint
	schoolErr  error
	invisible  bool
	rows       []repositories.GraduateOutcomeRow
	positions  []repositories.GraduatePositionCount
	optionsErr error
	gotSchool  uint
	gotFilter  repositories.GraduateOutcomeFilter
}

func (f *fakeStore) StudentSchoolID(uint) (*uint, error) { return f.school, f.schoolErr }
func (f *fakeStore) CompanyVisible(uint) (bool, error)   { return !f.invisible, nil }
func (f *fakeStore) CountsByCompanyYear(schoolID uint, filter repositories.GraduateOutcomeFilter) ([]repositories.GraduateOutcomeRow, error) {
	f.gotSchool, f.gotFilter = schoolID, filter
	return f.rows, nil
}
func (f *fakeStore) FilterOptions(uint) ([]int, []string, error) {
	return []int{2025, 2024}, []string{"製造業", "情報通信業"}, f.optionsErr
}
func (f *fakeStore) CompanyCounts(schoolID, companyID uint) ([]repositories.GraduateOutcomeRow, []repositories.GraduatePositionCount, error) {
	f.gotSchool = schoolID
	return f.rows, f.positions, nil
}

func linked() *uint {
	s := uint(5)
	return &s
}

func row(company uint, name string, year, n int) repositories.GraduateOutcomeRow {
	return repositories.GraduateOutcomeRow{CompanyID: company, CompanyName: name, GraduationYear: year, Count: n}
}

func TestList(t *testing.T) {
	store := &fakeStore{school: linked(), rows: []repositories.GraduateOutcomeRow{
		row(1, "B社", 2024, 3), row(2, "A社", 2025, 3), row(1, "B社", 2025, 1),
		row(3, "D社", 2025, 1), row(4, "C社", 2025, 2),
	}}
	out, err := NewService(store).List(9, Filter{Year: 2025, Industry: "製造業"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if store.gotSchool != 5 || store.gotFilter.Year != 2025 || store.gotFilter.Industry != "製造業" {
		t.Fatalf("学生の学校で絞っていない: school=%d filter=%+v", store.gotSchool, store.gotFilter)
	}
	if !out.SchoolLinked || out.Total != 10 || len(out.Companies) != 4 {
		t.Fatalf("out = %+v", out)
	}
	// 人数の多い順。少数（3人未満）の企業は人数が並びから分からないよう、末尾に企業名順
	want := []string{"B社", "A社", "C社", "D社"}
	for i, c := range out.Companies {
		if c.CompanyName != want[i] {
			t.Fatalf("並び = %+v", out.Companies)
		}
	}
	b := out.Companies[0]
	if *b.Total != 4 || len(b.ByYear) != 1 || b.ByYear[0].Year != 2024 || !b.ByYearFew {
		t.Fatalf("B社 = %+v（3人未満の年度は少数にまとめる）", b)
	}
	if c := out.Companies[2]; c.Total != nil || len(c.ByYear) != 0 {
		t.Fatalf("C社 = %+v（企業合計も3人未満なら人数を出さない）", c)
	}
}

// 最小人数の境界: 1・k-1 は少数、k は人数を出す。
func TestList_最小人数の境界(t *testing.T) {
	for _, tt := range []struct {
		n         int
		wantShown bool
	}{{1, false}, {MinCount - 1, false}, {MinCount, true}} {
		store := &fakeStore{school: linked(), rows: []repositories.GraduateOutcomeRow{row(1, "A社", 2025, tt.n)}}
		out, err := NewService(store).List(9, Filter{})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		c := out.Companies[0]
		if shown := c.Total != nil; shown != tt.wantShown {
			t.Errorf("n=%d: total=%v", tt.n, c.Total)
		}
		if tt.wantShown && (len(c.ByYear) != 1 || c.ByYear[0].Count != tt.n || c.ByYearFew) {
			t.Errorf("n=%d: by_year=%+v few=%v", tt.n, c.ByYear, c.ByYearFew)
		}
		if !tt.wantShown && len(c.ByYear) != 0 {
			t.Errorf("n=%d: 少数なのに年度別を出している %+v", tt.n, c.ByYear)
		}
	}
}

func TestList_学校が未登録なら実績を出さない(t *testing.T) {
	out, err := NewService(&fakeStore{}).List(9, Filter{})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out.SchoolLinked || len(out.Companies) != 0 || out.Companies == nil {
		t.Fatalf("out = %+v", out)
	}
}

func TestList_エラー(t *testing.T) {
	out, err := NewService(&fakeStore{schoolErr: repositories.ErrGraduateOutcomeUserNotFound}).List(9, Filter{})
	if !errors.Is(err, ErrUserNotFound) || out != nil {
		t.Fatalf("存在しない学生: out=%+v err=%v", out, err)
	}
	boom := errors.New("db")
	if out, err := NewService(&fakeStore{schoolErr: boom}).List(9, Filter{}); !errors.Is(err, boom) || out != nil {
		t.Fatalf("学校の取得に失敗: out=%+v err=%v", out, err)
	}
	if out, err := NewService(&fakeStore{school: linked(), optionsErr: boom}).List(9, Filter{}); !errors.Is(err, boom) || out != nil {
		t.Fatalf("選択肢の取得に失敗: out=%+v err=%v", out, err)
	}
}

func TestCompany(t *testing.T) {
	store := &fakeStore{
		school: linked(),
		rows:   []repositories.GraduateOutcomeRow{row(1, "A社", 2025, 3), row(1, "A社", 0, 3), row(1, "A社", 2024, 1)},
		positions: []repositories.GraduatePositionCount{
			{Title: "SE", Count: 4}, {Title: "", Count: 3}, {Title: "営業", Count: 2},
		},
	}
	out, err := NewService(store).Company(9, 1)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if store.gotSchool != 5 || *out.Total != 7 {
		t.Fatalf("out = %+v", out)
	}
	// 年度未登録（0）は最後。3人未満の年度・職種は少数にまとめる
	if len(out.ByYear) != 2 || out.ByYear[0].Year != 2025 || out.ByYear[1].Year != 0 || !out.ByYearFew {
		t.Fatalf("by_year = %+v few=%v", out.ByYear, out.ByYearFew)
	}
	if len(out.ByPosition) != 2 || out.ByPosition[1].Title != "職種の登録なし" || !out.ByPositionFew {
		t.Fatalf("by_position = %+v few=%v", out.ByPosition, out.ByPositionFew)
	}
}

func TestCompany_最小人数の境界(t *testing.T) {
	for _, n := range []int{1, MinCount - 1} {
		store := &fakeStore{school: linked(), rows: []repositories.GraduateOutcomeRow{row(1, "A社", 2025, n)},
			positions: []repositories.GraduatePositionCount{{Title: "SE", Count: n}}}
		out, err := NewService(store).Company(9, 1)
		if err != nil || out.Total != nil || len(out.ByYear) != 0 || len(out.ByPosition) != 0 {
			t.Fatalf("n=%d: 少数なのに人数を出している out=%+v err=%v", n, out, err)
		}
	}
	store := &fakeStore{school: linked(), rows: []repositories.GraduateOutcomeRow{row(1, "A社", 2025, MinCount)},
		positions: []repositories.GraduatePositionCount{{Title: "SE", Count: MinCount}}}
	out, err := NewService(store).Company(9, 1)
	if err != nil || *out.Total != MinCount || out.ByYear[0].Count != MinCount || out.ByPosition[0].Count != MinCount || out.ByYearFew || out.ByPositionFew {
		t.Fatalf("n=k: out=%+v err=%v", out, err)
	}
}

func TestCompany_実績なし_企業なし_学生なし(t *testing.T) {
	out, err := NewService(&fakeStore{school: linked()}).Company(9, 1)
	if err != nil || out.Total == nil || *out.Total != 0 {
		t.Fatalf("実績なしは 0 人: out=%+v err=%v", out, err)
	}
	if _, err := NewService(&fakeStore{school: linked(), invisible: true}).Company(9, 1); !errors.Is(err, ErrCompanyNotFound) {
		t.Fatalf("見せない企業: err=%v", err)
	}
	if _, err := NewService(&fakeStore{schoolErr: repositories.ErrGraduateOutcomeUserNotFound}).Company(9, 1); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("存在しない学生: err=%v", err)
	}
}
