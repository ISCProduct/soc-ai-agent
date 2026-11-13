package graduateoutcome

import (
	"testing"

	"Backend/internal/repositories"
)

// #2032: 学生向けの卒業生の進路実績。

type fakeStore struct {
	school    *uint
	gotSchool uint
	gotFilter repositories.GraduateOutcomeFilter
}

func (f *fakeStore) StudentSchoolID(uint) (*uint, error) { return f.school, nil }
func (f *fakeStore) CountsByCompanyYear(schoolID uint, filter repositories.GraduateOutcomeFilter) ([]repositories.GraduateOutcomeRow, error) {
	f.gotSchool, f.gotFilter = schoolID, filter
	return []repositories.GraduateOutcomeRow{
		{CompanyID: 1, CompanyName: "B社", Industry: "情報通信業", GraduationYear: 2024, Count: 1},
		{CompanyID: 2, CompanyName: "A社", Industry: "製造業", GraduationYear: 2025, Count: 2},
		{CompanyID: 1, CompanyName: "B社", Industry: "情報通信業", GraduationYear: 2025, Count: 1},
	}, nil
}
func (f *fakeStore) FilterOptions(uint) ([]int, []string, error) {
	return []int{2025, 2024}, []string{"製造業", "情報通信業"}, nil
}
func (f *fakeStore) CompanyCounts(schoolID, companyID uint) ([]repositories.GraduateOutcomeRow, []repositories.GraduatePositionCount, error) {
	f.gotSchool = schoolID
	return []repositories.GraduateOutcomeRow{{GraduationYear: 2025, Count: 2}, {GraduationYear: 2024, Count: 1}},
		[]repositories.GraduatePositionCount{{Title: "SE", Count: 2}, {Title: "", Count: 1}}, nil
}

func TestList(t *testing.T) {
	school := uint(5)
	store := &fakeStore{school: &school}
	out, err := NewService(store).List(9, repositories.GraduateOutcomeFilter{Year: 2025})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if store.gotSchool != 5 || store.gotFilter.Year != 2025 {
		t.Fatalf("学生の学校で絞っていない: school=%d filter=%+v", store.gotSchool, store.gotFilter)
	}
	if !out.SchoolLinked || out.Total != 4 || len(out.Companies) != 2 {
		t.Fatalf("out = %+v", out)
	}
	// 人数が同じ（2人）なら企業名順。推薦の優遇ではなく事実の並び
	if out.Companies[0].CompanyName != "A社" || out.Companies[1].CompanyName != "B社" {
		t.Fatalf("並び = %+v", out.Companies)
	}
	if b := out.Companies[1]; b.Total != 2 || b.ByYear[0].Year != 2025 {
		t.Fatalf("B社 = %+v（年度は新しい順）", b)
	}
}

func TestList_学校が未登録なら実績を出さない(t *testing.T) {
	out, err := NewService(&fakeStore{}).List(9, repositories.GraduateOutcomeFilter{})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out.SchoolLinked || len(out.Companies) != 0 || out.Companies == nil {
		t.Fatalf("out = %+v", out)
	}
}

func TestCompany(t *testing.T) {
	school := uint(5)
	out, err := NewService(&fakeStore{school: &school}).Company(9, 1)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out.Total != 3 || out.ByPosition[1].Title != "職種の登録なし" {
		t.Fatalf("out = %+v", out)
	}
}
