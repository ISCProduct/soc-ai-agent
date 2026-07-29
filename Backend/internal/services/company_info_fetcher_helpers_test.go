package services

import "testing"

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"全て空", []string{"", "  ", ""}, ""},
		{"最初が有効", []string{"a", "b"}, "a"},
		{"2番目が有効", []string{"", "b"}, "b"},
		{"空白スキップ", []string{"  ", "c"}, "c"},
		{"引数なし", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstNonEmpty(tt.values...); got != tt.want {
				t.Errorf("firstNonEmpty(%v) = %q, want %q", tt.values, got, tt.want)
			}
		})
	}
}

func TestGbizResultUseful(t *testing.T) {
	tests := []struct {
		name string
		r    *CompanyInfoResult
		want bool
	}{
		{"nil", nil, false},
		{"空結果", &CompanyInfoResult{}, false},
		{"Locationあり", &CompanyInfoResult{Location: "東京"}, true},
		{"WebsiteURLあり", &CompanyInfoResult{WebsiteURL: "https://example.com"}, true},
		{"EmployeeCountあり", &CompanyInfoResult{EmployeeCount: 100}, true},
		{"FoundedYearあり", &CompanyInfoResult{FoundedYear: 2000}, true},
		{"Descriptionあり", &CompanyInfoResult{Description: "概要"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gbizResultUseful(tt.r); got != tt.want {
				t.Errorf("gbizResultUseful() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeCompanyInfoGaps(t *testing.T) {
	base := &CompanyInfoResult{
		Description: "既存概要",
		Industry:    "",
		Location:    "東京",
		FoundedYear: 0,
	}
	ai := &CompanyInfoResult{
		Description: "AI概要",
		Industry:    "IT",
		Location:    "大阪",
		FoundedYear: 1990,
		MainBusiness: "ソフトウェア開発",
	}
	mergeCompanyInfoGaps(base, ai)

	if base.Description != "既存概要" {
		t.Errorf("既存フィールドが上書きされた: %q", base.Description)
	}
	if base.Industry != "IT" {
		t.Errorf("空欄が埋められなかった: Industry=%q", base.Industry)
	}
	if base.Location != "東京" {
		t.Errorf("既存Locationが上書きされた: %q", base.Location)
	}
	if base.FoundedYear != 1990 {
		t.Errorf("FoundedYearが埋められなかった: %d", base.FoundedYear)
	}
	if base.MainBusiness != "ソフトウェア開発" {
		t.Errorf("MainBusinessが埋められなかった: %q", base.MainBusiness)
	}
}
