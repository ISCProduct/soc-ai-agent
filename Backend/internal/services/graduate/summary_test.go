package graduate

import "testing"

func TestSummarize_内訳は登録件数に対する割合で氏名は持たない(t *testing.T) {
	got := Summarize([]Placement{
		{GraduationYear: 2024, SchoolName: "A校", Industry: "IT"},
		{GraduationYear: 2024, SchoolName: "A校", Industry: "IT"},
		{GraduationYear: 2025, SchoolName: "B校", Industry: ""},
		{GraduationYear: 0, SchoolName: "", Industry: "製造"},
	})
	if got.OfferRateAvailable {
		t.Fatal("卒業生全体の人数が無いのに内定率を出せる扱いになっている")
	}
	if got.RecordedCount != 4 {
		t.Fatalf("recorded = %d", got.RecordedCount)
	}
	if len(got.ByYear) != 3 || got.ByYear[0].Label != "2024卒" || got.ByYear[0].Count != 2 || got.ByYear[0].SharePercent != 50 {
		t.Fatalf("by year = %+v", got.ByYear)
	}
	if got.ByIndustry[0].Label != "IT" || got.ByIndustry[0].Count != 2 {
		t.Fatalf("by industry = %+v", got.ByIndustry)
	}
	foundUnset := false
	for _, b := range got.ByIndustry {
		if b.Label == "業界未設定" && b.Count == 1 {
			foundUnset = true
		}
	}
	if !foundUnset {
		t.Fatalf("空の業界が落ちている: %+v", got.ByIndustry)
	}
	if got.BySchool[0].Label != "A校" || got.BySchool[0].Count != 2 {
		t.Fatalf("by school = %+v", got.BySchool)
	}
}

func TestSummarize_0件でも内訳は空配列(t *testing.T) {
	got := Summarize(nil)
	if got.RecordedCount != 0 || got.ByYear == nil || got.ByIndustry == nil || got.BySchool == nil {
		t.Fatalf("%+v", got)
	}
}
