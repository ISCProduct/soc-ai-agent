package graduateoutcome

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"Backend/internal/repositories"
)

// #2032: 学生向けの卒業生の進路実績。

type fakeStore struct {
	school     *uint
	schoolErr  error
	invisible  bool
	rows       []repositories.GraduateOutcomeRow
	cellsErr   error
	optionsErr error
	gotSchool  uint
	gotFilter  repositories.GraduateOutcomeFilter
}

func (f *fakeStore) StudentSchoolID(context.Context, uint) (*uint, error) {
	return f.school, f.schoolErr
}
func (f *fakeStore) CompanyVisible(context.Context, uint) (bool, error) { return !f.invisible, nil }

// OutcomeCells は本物と同じく、最小の区分を企業・年度で絞って返す（業界は行に持たせない）。
func (f *fakeStore) OutcomeCells(_ context.Context, schoolID uint, filter repositories.GraduateOutcomeFilter) ([]repositories.GraduateOutcomeRow, error) {
	f.gotSchool, f.gotFilter = schoolID, filter
	if f.cellsErr != nil {
		return nil, f.cellsErr
	}
	var out []repositories.GraduateOutcomeRow
	for _, r := range f.rows {
		if (filter.CompanyID == 0 || r.CompanyID == filter.CompanyID) && (filter.Year == 0 || r.GraduationYear == filter.Year) {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeStore) FilterOptions(context.Context, uint) ([]int, []string, error) {
	return []int{2025, 2024}, []string{"製造業", "情報通信業"}, f.optionsErr
}

func linked() *uint {
	s := uint(5)
	return &s
}

// cell は最小の区分（企業 × 年度 × 職種）の人数。
func cell(company uint, name string, year int, title string, n int) repositories.GraduateOutcomeRow {
	return repositories.GraduateOutcomeRow{CompanyID: company, CompanyName: name, GraduationYear: year, Title: title, Count: n}
}

func TestList(t *testing.T) {
	store := &fakeStore{school: linked(), rows: []repositories.GraduateOutcomeRow{
		cell(1, "B社", 2024, "SE", 3), cell(1, "B社", 2025, "SE", 4), cell(1, "B社", 2025, "営業", 1),
		cell(2, "A社", 2025, "SE", 3),
		cell(3, "D社", 2025, "SE", 1), cell(4, "C社", 2025, "SE", 2), cell(4, "C社", 2024, "営業", 2),
	}}
	out, err := NewService(store).List(context.Background(), 9, Filter{Industry: "製造業"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if store.gotSchool != 5 || store.gotFilter.Industry != "製造業" || store.gotFilter.CompanyID != 0 {
		t.Fatalf("学生の学校で絞っていない: school=%d filter=%+v", store.gotSchool, store.gotFilter)
	}
	// 合計は見せる区分の和（3+4+3）。少数の区分（1・1・2・2）は入れない
	if !out.SchoolLinked || out.Total != 10 || !out.TotalFew || len(out.Companies) != 4 {
		t.Fatalf("out = %+v", out)
	}
	// 人数の多い順。見せる区分の無い企業は、末尾に企業名順
	want := []string{"B社", "A社", "C社", "D社"}
	for i, c := range out.Companies {
		if c.CompanyName != want[i] {
			t.Fatalf("並び = %+v", out.Companies)
		}
	}
	b := out.Companies[0]
	if *b.Total != 7 || !b.TotalFew || !reflect.DeepEqual(b.ByYear, []YearCount{{2025, 4}, {2024, 3}}) || !b.ByYearFew {
		t.Fatalf("B社 = %+v（営業 1 人は数に入れず「＋少数」）", b)
	}
	if a := out.Companies[1]; *a.Total != 3 || a.TotalFew || a.ByYearFew {
		t.Fatalf("A社 = %+v", a)
	}
	// C社は合計 4 人だが、どの区分も 3 人未満なので人数を出さない
	if c := out.Companies[2]; c.Total != nil || len(c.ByYear) != 0 || !c.TotalFew {
		t.Fatalf("C社 = %+v", c)
	}
}

// 最小人数の境界: 1・k-1 は少数、k は人数を出す。
func TestList_最小人数の境界(t *testing.T) {
	for _, tt := range []struct {
		n         int
		wantShown bool
	}{{1, false}, {MinCount - 1, false}, {MinCount, true}} {
		store := &fakeStore{school: linked(), rows: []repositories.GraduateOutcomeRow{cell(1, "A社", 2025, "SE", tt.n)}}
		out, err := NewService(store).List(context.Background(), 9, Filter{})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		c := out.Companies[0]
		if shown := c.Total != nil; shown != tt.wantShown {
			t.Errorf("n=%d: total=%v", tt.n, c.Total)
		}
		if tt.wantShown && (len(c.ByYear) != 1 || c.ByYear[0].Count != tt.n || c.ByYearFew || c.TotalFew || out.TotalFew) {
			t.Errorf("n=%d: out=%+v", tt.n, out)
		}
		if !tt.wantShown && (len(c.ByYear) != 0 || out.Total != 0) {
			t.Errorf("n=%d: 少数なのに人数を出している %+v", tt.n, out)
		}
	}
}

// 少数の区分の人数を変えても、画面に出る数字はどれも変わらない（隠した人数がどの数字にも入っていない）。
// 合計からの引き算・年度別と職種別の比較・年度での絞り込みの前後の比較で、隠した人数が求まらないことの根拠。
func TestHiddenCountsNeverAffectShownNumbers(t *testing.T) {
	base := func(hiddenA, hiddenB int) []repositories.GraduateOutcomeRow {
		return []repositories.GraduateOutcomeRow{
			cell(1, "A社", 2025, "SE", 5), cell(1, "A社", 2024, "SE", 3), cell(1, "A社", 2024, "営業", 4),
			cell(1, "A社", 2025, "営業", hiddenA), // 隠す区分（2025 × 営業）
			cell(1, "A社", 0, "", hiddenB),      // 隠す区分（年度・職種の登録なし）
		}
	}
	type snapshot struct {
		list, list2025, list2024 *List
		detail                   *CompanyDetail
	}
	take := func(rows []repositories.GraduateOutcomeRow) snapshot {
		svc := NewService(&fakeStore{school: linked(), rows: rows})
		ctx := context.Background()
		l, _ := svc.List(ctx, 9, Filter{})
		l25, _ := svc.List(ctx, 9, Filter{Year: 2025})
		l24, _ := svc.List(ctx, 9, Filter{Year: 2024})
		d, _ := svc.Company(ctx, 9, 1)
		return snapshot{l, l25, l24, d}
	}
	one, two := take(base(1, 1)), take(base(2, 2))
	if !reflect.DeepEqual(one, two) {
		t.Fatalf("隠した人数で表示が変わる:\n1人: %+v\n2人: %+v", one, two)
	}
	d := one.detail
	// 合計 = 年度別の和 = 職種別の和（引き算で差が出ない）
	sumYears, sumPositions := 0, 0
	for _, y := range d.ByYear {
		sumYears += y.Count
	}
	for _, p := range d.ByPosition {
		sumPositions += p.Count
	}
	if *d.Total != 12 || sumYears != 12 || sumPositions != 12 || !d.TotalFew {
		t.Fatalf("detail = %+v（合計 %d / 年度別 %d / 職種別 %d）", d, *d.Total, sumYears, sumPositions)
	}
	// 年度で絞り込んだ合計は、絞り込む前の年度別と一致する
	if one.list2025.Total != 5 || one.list2024.Total != 7 || one.list.Total != 12 {
		t.Fatalf("絞り込み: all=%d 2025=%d 2024=%d", one.list.Total, one.list2025.Total, one.list2024.Total)
	}
	// 隠す区分が無ければ「＋少数」も立たない
	if none := take(base(MinCount, MinCount)).detail; none.TotalFew || none.ByYearFew || none.ByPositionFew {
		t.Fatalf("少数が無いのに few が立つ: %+v", none)
	}
}

func TestList_学校が未登録なら実績を出さない(t *testing.T) {
	out, err := NewService(&fakeStore{}).List(context.Background(), 9, Filter{})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out.SchoolLinked || len(out.Companies) != 0 || out.Companies == nil {
		t.Fatalf("out = %+v", out)
	}
}

func TestList_エラー(t *testing.T) {
	out, err := NewService(&fakeStore{schoolErr: repositories.ErrGraduateOutcomeUserNotFound}).List(context.Background(), 9, Filter{})
	if !errors.Is(err, ErrUserNotFound) || out != nil {
		t.Fatalf("存在しない学生: out=%+v err=%v", out, err)
	}
	boom := errors.New("db")
	if out, err := NewService(&fakeStore{schoolErr: boom}).List(context.Background(), 9, Filter{}); !errors.Is(err, boom) || out != nil {
		t.Fatalf("学校の取得に失敗: out=%+v err=%v", out, err)
	}
	if out, err := NewService(&fakeStore{school: linked(), cellsErr: boom}).List(context.Background(), 9, Filter{}); !errors.Is(err, boom) || out != nil {
		t.Fatalf("集計に失敗: out=%+v err=%v", out, err)
	}
	if out, err := NewService(&fakeStore{school: linked(), optionsErr: boom}).List(context.Background(), 9, Filter{}); !errors.Is(err, boom) || out != nil {
		t.Fatalf("選択肢の取得に失敗: out=%+v err=%v", out, err)
	}
}

func TestCompany(t *testing.T) {
	store := &fakeStore{
		school: linked(),
		rows: []repositories.GraduateOutcomeRow{
			cell(1, "A社", 2025, "SE", 4), cell(1, "A社", 0, "", 3), cell(1, "A社", 2024, "営業", 2),
			cell(2, "B社", 2025, "SE", 9),
		},
	}
	out, err := NewService(store).Company(context.Background(), 9, 1)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if store.gotSchool != 5 || store.gotFilter.CompanyID != 1 || *out.Total != 7 || !out.TotalFew {
		t.Fatalf("out = %+v filter=%+v", out, store.gotFilter)
	}
	// 年度未登録（0）は最後。3人未満の区分は数に入れず少数にまとめる
	if !reflect.DeepEqual(out.ByYear, []YearCount{{2025, 4}, {0, 3}}) || !out.ByYearFew {
		t.Fatalf("by_year = %+v few=%v", out.ByYear, out.ByYearFew)
	}
	if !reflect.DeepEqual(out.ByPosition, []PositionCount{{"SE", 4}, {"職種の登録なし", 3}}) || !out.ByPositionFew {
		t.Fatalf("by_position = %+v few=%v", out.ByPosition, out.ByPositionFew)
	}
}

func TestCompany_最小人数の境界(t *testing.T) {
	for _, n := range []int{1, MinCount - 1} {
		store := &fakeStore{school: linked(), rows: []repositories.GraduateOutcomeRow{cell(1, "A社", 2025, "SE", n)}}
		out, err := NewService(store).Company(context.Background(), 9, 1)
		if err != nil || out.Total != nil || len(out.ByYear) != 0 || len(out.ByPosition) != 0 || !out.TotalFew {
			t.Fatalf("n=%d: 少数なのに人数を出している out=%+v err=%v", n, out, err)
		}
	}
	store := &fakeStore{school: linked(), rows: []repositories.GraduateOutcomeRow{cell(1, "A社", 2025, "SE", MinCount)}}
	out, err := NewService(store).Company(context.Background(), 9, 1)
	if err != nil || *out.Total != MinCount || out.ByYear[0].Count != MinCount || out.ByPosition[0].Count != MinCount ||
		out.TotalFew || out.ByYearFew || out.ByPositionFew {
		t.Fatalf("n=k: out=%+v err=%v", out, err)
	}
}

func TestCompany_実績なし_企業なし_学生なし(t *testing.T) {
	out, err := NewService(&fakeStore{school: linked()}).Company(context.Background(), 9, 1)
	if err != nil || out.Total == nil || *out.Total != 0 || out.TotalFew {
		t.Fatalf("実績なしは 0 人: out=%+v err=%v", out, err)
	}
	if _, err := NewService(&fakeStore{school: linked(), invisible: true}).Company(context.Background(), 9, 1); !errors.Is(err, ErrCompanyNotFound) {
		t.Fatalf("見せない企業: err=%v", err)
	}
	if _, err := NewService(&fakeStore{schoolErr: repositories.ErrGraduateOutcomeUserNotFound}).Company(context.Background(), 9, 1); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("存在しない学生: err=%v", err)
	}
	boom := errors.New("db")
	if out, err := NewService(&fakeStore{school: linked(), cellsErr: boom}).Company(context.Background(), 9, 1); !errors.Is(err, boom) || out != nil {
		t.Fatalf("集計に失敗: out=%+v err=%v", out, err)
	}
}
