package services

import (
	"Backend/internal/models"
	"strings"
	"testing"
)

func TestParseMynaviCompanyPage_TableFormat(t *testing.T) {
	mockHTML := `<!DOCTYPE html>
<html>
<head><title>テスト株式会社 | マイナビ</title></head>
<body>
<h1 class="companyName">テスト株式会社</h1>
<table class="companyInfoTable">
  <tr><th>業種</th><td>IT・ソフトウェア</td></tr>
  <tr><th>設立</th><td>2005年4月</td></tr>
  <tr><th>従業員数</th><td>1,200名</td></tr>
  <tr><th>本社所在地</th><td>東京都渋谷区</td></tr>
  <tr><th>事業内容</th><td>クラウドサービスの開発・運営</td></tr>
  <tr><th>企業概要</th><td>先進的なSaaS企業</td></tr>
  <tr><th>ホームページ</th><td>https://test-company.example.com</td></tr>
  <tr><th>福利厚生</th><td>各種社会保険、リモートワーク可</td></tr>
  <tr><th>社風</th><td>フラットな組織文化</td></tr>
  <tr><th>平均年齢</th><td>32.5歳</td></tr>
  <tr><th>女性比率</th><td>42.0%</td></tr>
</table>
</body>
</html>`

	data, err := parseMynaviCompanyPage([]byte(mockHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stringTests := []struct {
		field string
		got   string
		want  string
	}{
		{"Name", data.Name, "テスト株式会社"},
		{"Industry", data.Industry, "IT・ソフトウェア"},
		{"Location", data.Location, "東京都渋谷区"},
		{"MainBusiness", data.MainBusiness, "クラウドサービスの開発・運営"},
		{"Description", data.Description, "先進的なSaaS企業"},
		{"WebsiteURL", data.WebsiteURL, "https://test-company.example.com"},
		{"WelfareDetails", data.WelfareDetails, "各種社会保険、リモートワーク可"},
		{"Culture", data.Culture, "フラットな組織文化"},
	}
	for _, tc := range stringTests {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.field, tc.got, tc.want)
		}
	}
	if data.FoundedYear != 2005 {
		t.Errorf("FoundedYear: got %d, want 2005", data.FoundedYear)
	}
	if data.EmployeeCount != 1200 {
		t.Errorf("EmployeeCount: got %d, want 1200", data.EmployeeCount)
	}
	if data.AverageAge != 32.5 {
		t.Errorf("AverageAge: got %f, want 32.5", data.AverageAge)
	}
	if data.FemaleRatio != 42.0 {
		t.Errorf("FemaleRatio: got %f, want 42.0", data.FemaleRatio)
	}
}

func TestParseMynaviCompanyPage_DLFormat(t *testing.T) {
	mockHTML := `<!DOCTYPE html>
<html>
<head><title>株式会社サンプル | マイナビ2026</title></head>
<body>
<h1>株式会社サンプル</h1>
<dl class="companyInfo">
  <dt>業界</dt><dd>製造業</dd>
  <dt>創業</dt><dd>1998年10月</dd>
  <dt>社員数</dt><dd>500人</dd>
  <dt>所在地</dt><dd>大阪府大阪市</dd>
  <dt>主要事業</dt><dd>精密機器の製造・販売</dd>
  <dt>平均年齢</dt><dd>38歳</dd>
  <dt>女性社員</dt><dd>25%</dd>
</dl>
</body>
</html>`

	data, err := parseMynaviCompanyPage([]byte(mockHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		field string
		got   string
		want  string
	}{
		{"Name", data.Name, "株式会社サンプル"},
		{"Industry", data.Industry, "製造業"},
		{"Location", data.Location, "大阪府大阪市"},
		{"MainBusiness", data.MainBusiness, "精密機器の製造・販売"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.field, tc.got, tc.want)
		}
	}
	if data.FoundedYear != 1998 {
		t.Errorf("FoundedYear: got %d, want 1998", data.FoundedYear)
	}
	if data.EmployeeCount != 500 {
		t.Errorf("EmployeeCount: got %d, want 500", data.EmployeeCount)
	}
	if data.AverageAge != 38.0 {
		t.Errorf("AverageAge: got %f, want 38.0", data.AverageAge)
	}
	if data.FemaleRatio != 25.0 {
		t.Errorf("FemaleRatio: got %f, want 25.0", data.FemaleRatio)
	}
}

func TestParseMynaviCompanyPage_TitleFallback(t *testing.T) {
	mockHTML := `<!DOCTYPE html>
<html>
<head><title>フォールバック企業 | マイナビ2026</title></head>
<body><p>最低限のページ</p></body>
</html>`

	data, err := parseMynaviCompanyPage([]byte(mockHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Name != "フォールバック企業" {
		t.Errorf("Name from title fallback: got %q, want %q", data.Name, "フォールバック企業")
	}
}

func TestParseMynaviCompanyPage_EmptyHTML(t *testing.T) {
	data, err := parseMynaviCompanyPage([]byte(`<html><body></body></html>`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Name != "" {
		t.Errorf("Name should be empty for minimal HTML, got %q", data.Name)
	}
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"2005年4月", 2005},
		{"1998年10月設立", 1998},
		{"昭和50年（1975年）", 1975},
		{"不明", 0},
		{"", 0},
	}
	for _, tc := range tests {
		if got := extractYear(tc.input); got != tc.want {
			t.Errorf("extractYear(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestExtractInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"1,200名", 1200},
		{"500人", 500},
		{"約10000名", 10000},
		{"不明", 0},
		{"", 0},
	}
	for _, tc := range tests {
		if got := extractInt(tc.input); got != tc.want {
			t.Errorf("extractInt(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestExtractFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"32.5歳", 32.5},
		{"42%", 42.0},
		{"38", 38.0},
		{"不明", 0},
		{"", 0},
	}
	for _, tc := range tests {
		if got := extractFloat(tc.input); got != tc.want {
			t.Errorf("extractFloat(%q) = %f, want %f", tc.input, got, tc.want)
		}
	}
}

func TestExtractCharset(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{"text/html; charset=Shift_JIS", "shift_jis"},
		{"text/html; charset=shift-jis", "shift_jis"},
		{"text/html; charset=EUC-JP", "euc-jp"},
		{"text/html; charset=UTF-8", "utf-8"},
		{"text/html", "utf-8"},
	}
	for _, tc := range tests {
		if got := extractCharset(tc.contentType); got != tc.want {
			t.Errorf("extractCharset(%q) = %q, want %q", tc.contentType, got, tc.want)
		}
	}
}

func TestValidateCrawlSource_MynaviCompany(t *testing.T) {
	tests := []struct {
		name      string
		source    *models.CrawlSource
		wantErr   bool
		errSubstr string
	}{
		{
			name: "有効なmynavi_company",
			source: &models.CrawlSource{
				Name:         "マイナビテスト",
				TargetType:   "mynavi_company",
				SourceURL:    "https://job.mynavi.jp/26/pc/search/corp12345/outline.html",
				ScheduleType: "weekly",
				ScheduleDay:  1,
				ScheduleTime: "02:00",
			},
			wantErr: false,
		},
		{
			name: "mynavi_companyでURLなし",
			source: &models.CrawlSource{
				Name:         "URLなしテスト",
				TargetType:   "mynavi_company",
				SourceURL:    "",
				ScheduleType: "weekly",
				ScheduleDay:  1,
				ScheduleTime: "02:00",
			},
			wantErr:   true,
			errSubstr: "source_url is required for mynavi_company",
		},
		{
			name: "無効なtarget_type",
			source: &models.CrawlSource{
				Name:         "無効タイプ",
				TargetType:   "invalid_type",
				SourceURL:    "https://example.com",
				ScheduleType: "weekly",
				ScheduleDay:  1,
				ScheduleTime: "02:00",
			},
			wantErr:   true,
			errSubstr: "target_type must be",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCrawlSource(tc.source)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errSubstr)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
