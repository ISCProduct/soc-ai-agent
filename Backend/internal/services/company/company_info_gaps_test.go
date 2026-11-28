package company

import "testing"

// 社風・働き方が空でも、基本情報がそろっていれば Web検索で補わない（SOCAIAGENT-723）。
func TestCompanyInfoHasGaps(t *testing.T) {
	full := CompanyInfoResult{Description: "概要", MainBusiness: "事業", Industry: "IT", WebsiteURL: "https://example.com"}
	tests := []struct {
		name string
		edit func(r *CompanyInfoResult)
		want bool
	}{
		{name: "基本情報がそろっている", edit: func(r *CompanyInfoResult) {}, want: false},
		{name: "社風・働き方が空でも補わない", edit: func(r *CompanyInfoResult) { r.Culture, r.WorkStyle = "", "" }, want: false},
		{name: "事業内容が空なら補う", edit: func(r *CompanyInfoResult) { r.MainBusiness = "" }, want: true},
		{name: "公式サイトが空なら補う", edit: func(r *CompanyInfoResult) { r.WebsiteURL = "" }, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := full
			tt.edit(&r)
			if got := companyInfoHasGaps(&r); got != tt.want {
				t.Errorf("companyInfoHasGaps() = %v, want %v", got, tt.want)
			}
		})
	}
}
