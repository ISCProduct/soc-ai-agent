package services

import (
	"Backend/internal/models"
	"strings"
	"testing"
)

func TestParseOpenworkCompanyPage_FullScores(t *testing.T) {
	mockHTML := `<!DOCTYPE html>
<html>
<head><title>テスト株式会社 の評判・口コミ | OpenWork</title></head>
<body>
<h1 class="p-companyTop__name">テスト株式会社</h1>
<span class="p-companyTop__industry">IT・ソフトウェア</span>
<span class="p-totalScore__value">3.8</span>
<dl class="p-scoreList">
  <dt>待遇面の満足度</dt><dd>3.5</dd>
  <dt>社員の士気</dt><dd>4.0</dd>
  <dt>風通しの良さ</dt><dd>3.7</dd>
  <dt>20代成長環境</dt><dd>4.2</dd>
</dl>
<span class="p-averageSalary">平均年収 550万円</span>
<span class="p-reviewCount">口コミ 1,234件</span>
</body>
</html>`

	data, err := parseOpenworkCompanyPage([]byte(mockHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Name != "テスト株式会社" {
		t.Errorf("Name: got %q, want %q", data.Name, "テスト株式会社")
	}
	if data.Industry != "IT・ソフトウェア" {
		t.Errorf("Industry: got %q, want %q", data.Industry, "IT・ソフトウェア")
	}
	if data.Scores.Overall != 3.8 {
		t.Errorf("Overall: got %f, want 3.8", data.Scores.Overall)
	}
	if data.Scores.Welfare != 3.5 {
		t.Errorf("Welfare: got %f, want 3.5", data.Scores.Welfare)
	}
	if data.Scores.Morale != 4.0 {
		t.Errorf("Morale: got %f, want 4.0", data.Scores.Morale)
	}
	if data.Scores.Openness != 3.7 {
		t.Errorf("Openness: got %f, want 3.7", data.Scores.Openness)
	}
	if data.Scores.Growth != 4.2 {
		t.Errorf("Growth: got %f, want 4.2", data.Scores.Growth)
	}
	if data.AverageSalary != 550 {
		t.Errorf("AverageSalary: got %d, want 550", data.AverageSalary)
	}
	if data.ReviewCount != 1234 {
		t.Errorf("ReviewCount: got %d, want 1234", data.ReviewCount)
	}
}

func TestParseOpenworkCompanyPage_TitleFallback(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"の評判区切り", "株式会社ABC の評判・口コミ | OpenWork", "株式会社ABC"},
		{"パイプ区切り", "株式会社DEF | OpenWork", "株式会社DEF"},
		{"の口コミ区切り", "株式会社GHI の口コミ・評価 | OpenWork", "株式会社GHI"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			html := `<!DOCTYPE html><html><head><title>` + tc.title + `</title></head><body></body></html>`
			data, err := parseOpenworkCompanyPage([]byte(html))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if data.Name != tc.want {
				t.Errorf("Name: got %q, want %q", data.Name, tc.want)
			}
		})
	}
}

func TestParseOpenworkCompanyPage_EmptyHTML(t *testing.T) {
	data, err := parseOpenworkCompanyPage([]byte(`<html><body></body></html>`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Name != "" {
		t.Errorf("Name should be empty for minimal HTML, got %q", data.Name)
	}
	if data.Scores.Overall != 0 {
		t.Errorf("Overall score should be 0 for empty HTML, got %f", data.Scores.Overall)
	}
}

func TestParseOpenworkScore(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"3.8", 3.8},
		{"4.0", 4.0},
		{"0.0", 0.0},
		{"5", 5.0},
		{"5.0点", 5.0},
		{"不明", 0},
		{"", 0},
		{"6.0", 0}, // 5点満点を超えるため除外
	}
	for _, tc := range tests {
		if got := parseOpenworkScore(tc.input); got != tc.want {
			t.Errorf("parseOpenworkScore(%q) = %f, want %f", tc.input, got, tc.want)
		}
	}
}

func TestParseOpenworkSalary(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"平均年収 550万円", 550},
		{"550万円", 550},
		{"1,200万円", 1200},
		{"年収: 450万円", 450},
		{"不明", 0},
		{"", 0},
	}
	for _, tc := range tests {
		if got := parseOpenworkSalary(tc.input); got != tc.want {
			t.Errorf("parseOpenworkSalary(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestParseOpenworkReviewCount(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"口コミ 1,234件", 1234},
		{"234件の口コミ", 234},
		{"件数: 56件", 56},
		{"なし", 0},
		{"", 0},
	}
	for _, tc := range tests {
		if got := parseOpenworkReviewCount(tc.input); got != tc.want {
			t.Errorf("parseOpenworkReviewCount(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestBuildOpenworkCultureJSON(t *testing.T) {
	scores := openworkScores{Overall: 3.8, Morale: 4.0, Openness: 3.7, Growth: 4.2}
	result := buildOpenworkCultureJSON(scores)
	if result == "" {
		t.Fatal("expected non-empty JSON")
	}
	for _, key := range []string{"overall_score", "morale_score", "openness_score", "growth_score"} {
		if !strings.Contains(result, key) {
			t.Errorf("JSON missing key %q: %s", key, result)
		}
	}

	// 全スコアが0の場合は空文字列
	empty := buildOpenworkCultureJSON(openworkScores{})
	if empty != "" {
		t.Errorf("expected empty string for zero scores, got %q", empty)
	}
}

func TestBuildOpenworkWelfareText(t *testing.T) {
	got := buildOpenworkWelfareText(3.5)
	if !strings.Contains(got, "3.5") || !strings.Contains(got, "OpenWork") {
		t.Errorf("unexpected welfare text: %q", got)
	}
	if buildOpenworkWelfareText(0) != "" {
		t.Error("expected empty string for zero welfare score")
	}
}

func TestBuildOpenworkSalaryText(t *testing.T) {
	got := buildOpenworkSalaryText(550)
	if !strings.Contains(got, "550") || !strings.Contains(got, "OpenWork") {
		t.Errorf("unexpected salary text: %q", got)
	}
	if buildOpenworkSalaryText(0) != "" {
		t.Error("expected empty string for zero salary")
	}
}

func TestExtractOpenworkCharset(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{"text/html; charset=Shift_JIS", "shift_jis"},
		{"text/html; charset=EUC-JP", "euc-jp"},
		{"text/html; charset=UTF-8", "utf-8"},
		{"text/html", "utf-8"},
	}
	for _, tc := range tests {
		if got := extractOpenworkCharset(tc.contentType); got != tc.want {
			t.Errorf("extractOpenworkCharset(%q) = %q, want %q", tc.contentType, got, tc.want)
		}
	}
}

func TestValidateCrawlSource_OpenworkCompany(t *testing.T) {
	tests := []struct {
		name      string
		source    *models.CrawlSource
		wantErr   bool
		errSubstr string
	}{
		{
			name: "有効なopenwork_company",
			source: &models.CrawlSource{
				Name:         "テスト企業 (OpenWork)",
				TargetType:   "openwork_company",
				SourceURL:    "https://www.openwork.jp/company.php?co_no=a0210800005",
				ScheduleType: "monthly",
				ScheduleDay:  1,
				ScheduleTime: "04:00",
			},
			wantErr: false,
		},
		{
			name: "openwork_companyでURLなし",
			source: &models.CrawlSource{
				Name:         "URLなしテスト",
				TargetType:   "openwork_company",
				SourceURL:    "",
				ScheduleType: "monthly",
				ScheduleDay:  1,
				ScheduleTime: "04:00",
			},
			wantErr:   true,
			errSubstr: "source_url is required for openwork_company",
		},
		{
			name: "無効なtarget_type",
			source: &models.CrawlSource{
				Name:         "無効タイプ",
				TargetType:   "unknown_type",
				SourceURL:    "https://example.com",
				ScheduleType: "weekly",
				ScheduleDay:  1,
				ScheduleTime: "02:00",
			},
			wantErr:   true,
			errSubstr: "target_type must be",
		},
		{
			name: "openwork_companyはtarget_typeとして受け入れられる",
			source: &models.CrawlSource{
				Name:         "valid",
				TargetType:   "openwork_company",
				SourceURL:    "https://www.openwork.jp/company.php?co_no=test",
				ScheduleType: "weekly",
				ScheduleDay:  0,
				ScheduleTime: "03:00",
			},
			wantErr: false,
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
