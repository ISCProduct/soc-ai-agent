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
	tests := []struct {
		name string
		base *CompanyInfoResult
		ai   *CompanyInfoResult
		want *CompanyInfoResult
	}{
		{
			name: "既存値保持と空欄補完",
			base: &CompanyInfoResult{
				Description: "既存概要",
				Industry:    "",
				Location:    "東京",
				FoundedYear: 0,
			},
			ai: &CompanyInfoResult{
				Description:  "AI概要",
				Industry:     "IT",
				Location:     "大阪",
				FoundedYear:  1990,
				MainBusiness: "ソフトウェア開発",
			},
			want: &CompanyInfoResult{
				Description:  "既存概要",
				Industry:     "IT",
				Location:     "東京",
				FoundedYear:  1990,
				MainBusiness: "ソフトウェア開発",
			},
		},
		{
			name: "補完元が空",
			base: &CompanyInfoResult{
				Description: "既存概要",
				Location:    "東京",
			},
			ai: &CompanyInfoResult{},
			want: &CompanyInfoResult{
				Description: "既存概要",
				Location:    "東京",
			},
		},
		{
			name: "baseが全て空でAIから埋まる",
			base: &CompanyInfoResult{},
			ai: &CompanyInfoResult{
				Description:   "AI概要",
				Industry:      "IT",
				Location:      "大阪",
				WebsiteURL:    "https://example.com",
				FoundedYear:   2000,
				EmployeeCount: 50,
				MainBusiness:  "開発",
				Culture:       "フラット",
				WorkStyle:     "リモート",
				TechStack:     "Go",
				WelfareDetails: "住宅手当",
			},
			want: &CompanyInfoResult{
				Description:   "AI概要",
				Industry:      "IT",
				Location:      "大阪",
				WebsiteURL:    "https://example.com",
				FoundedYear:   2000,
				EmployeeCount: 50,
				MainBusiness:  "開発",
				Culture:       "フラット",
				WorkStyle:     "リモート",
				TechStack:     "Go",
				WelfareDetails: "住宅手当",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergeCompanyInfoGaps(tt.base, tt.ai)
			assertCompanyInfoResult(t, tt.base, tt.want)
		})
	}
}

func assertCompanyInfoResult(t *testing.T, got, want *CompanyInfoResult) {
	t.Helper()
	if got.Description != want.Description {
		t.Errorf("Description = %q, want %q", got.Description, want.Description)
	}
	if got.Industry != want.Industry {
		t.Errorf("Industry = %q, want %q", got.Industry, want.Industry)
	}
	if got.Location != want.Location {
		t.Errorf("Location = %q, want %q", got.Location, want.Location)
	}
	if got.WebsiteURL != want.WebsiteURL {
		t.Errorf("WebsiteURL = %q, want %q", got.WebsiteURL, want.WebsiteURL)
	}
	if got.FoundedYear != want.FoundedYear {
		t.Errorf("FoundedYear = %d, want %d", got.FoundedYear, want.FoundedYear)
	}
	if got.EmployeeCount != want.EmployeeCount {
		t.Errorf("EmployeeCount = %d, want %d", got.EmployeeCount, want.EmployeeCount)
	}
	if got.MainBusiness != want.MainBusiness {
		t.Errorf("MainBusiness = %q, want %q", got.MainBusiness, want.MainBusiness)
	}
	if got.Culture != want.Culture {
		t.Errorf("Culture = %q, want %q", got.Culture, want.Culture)
	}
	if got.WorkStyle != want.WorkStyle {
		t.Errorf("WorkStyle = %q, want %q", got.WorkStyle, want.WorkStyle)
	}
	if got.TechStack != want.TechStack {
		t.Errorf("TechStack = %q, want %q", got.TechStack, want.TechStack)
	}
	if got.WelfareDetails != want.WelfareDetails {
		t.Errorf("WelfareDetails = %q, want %q", got.WelfareDetails, want.WelfareDetails)
	}
}
