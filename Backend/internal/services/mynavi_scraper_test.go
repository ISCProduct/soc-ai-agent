package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMynaviCompanyPage(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		wantName string
		check    func(t *testing.T, d *MynaviCompanyData)
	}{
		{
			name: "dl形式の基本情報を正しく抽出できる",
			html: `<html><body>
				<h1 class="corp-head__name">株式会社サンプル</h1>
				<dl>
					<dt>業種</dt><dd>IT・通信</dd>
					<dt>従業員数</dt><dd>1,200名</dd>
					<dt>設立</dt><dd>2005年4月</dd>
					<dt>所在地</dt><dd>東京都渋谷区</dd>
					<dt>平均年齢</dt><dd>32.5歳</dd>
					<dt>女性比率</dt><dd>40.0%</dd>
					<dt>福利厚生</dt><dd>各種保険完備</dd>
					<dt>事業内容</dt><dd>クラウドサービスの開発・提供</dd>
				</dl>
			</body></html>`,
			wantName: "株式会社サンプル",
			check: func(t *testing.T, d *MynaviCompanyData) {
				assert.Equal(t, "IT・通信", d.Industry)
				assert.Equal(t, 1200, d.EmployeeCount)
				assert.Equal(t, 2005, d.FoundedYear)
				assert.Equal(t, "東京都渋谷区", d.Location)
				assert.InDelta(t, 32.5, d.AverageAge, 0.01)
				assert.InDelta(t, 40.0, d.FemaleRatio, 0.01)
				assert.Equal(t, "各種保険完備", d.WelfareDetails)
				assert.Equal(t, "クラウドサービスの開発・提供", d.MainBusiness)
			},
		},
		{
			name: "table形式の基本情報を正しく抽出できる",
			html: `<html><body>
				<h1>テーブル企業株式会社</h1>
				<table>
					<tr><th>業界</th><td>製造業</td></tr>
					<tr><th>社員数</th><td>500人</td></tr>
					<tr><th>設立</th><td>1998年</td></tr>
					<tr><th>本社所在地</th><td>大阪府大阪市</td></tr>
				</table>
			</body></html>`,
			wantName: "テーブル企業株式会社",
			check: func(t *testing.T, d *MynaviCompanyData) {
				assert.Equal(t, "製造業", d.Industry)
				assert.Equal(t, 500, d.EmployeeCount)
				assert.Equal(t, 1998, d.FoundedYear)
				assert.Equal(t, "大阪府大阪市", d.Location)
			},
		},
		{
			name: "企業名が取得できない場合は空文字を返す",
			html: `<html><body><p>情報なし</p></body></html>`,
			wantName: "",
			check:    func(t *testing.T, d *MynaviCompanyData) {},
		},
		{
			name: "カンマ付き従業員数を正しくパースできる",
			html: `<html><body>
				<h1>大企業株式会社</h1>
				<dl><dt>従業員数</dt><dd>12,345名</dd></dl>
			</body></html>`,
			wantName: "大企業株式会社",
			check: func(t *testing.T, d *MynaviCompanyData) {
				assert.Equal(t, 12345, d.EmployeeCount)
			},
		},
		{
			name: "和暦・複数フォーマットの設立年を正しくパースできる",
			html: `<html><body>
				<h1>老舗株式会社</h1>
				<dl><dt>設立</dt><dd>1975年（昭和50年）3月</dd></dl>
			</body></html>`,
			wantName: "老舗株式会社",
			check: func(t *testing.T, d *MynaviCompanyData) {
				assert.Equal(t, 1975, d.FoundedYear)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMynaviCompanyPage(tt.html)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantName, got.Name)
			tt.check(t, got)
		})
	}
}

func TestExtractInt(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"1,200名", 1200},
		{"500人", 500},
		{"12,345名", 12345},
		{"数字なし", 0},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, extractInt(tt.input), "input: %s", tt.input)
	}
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"2005年4月", 2005},
		{"1998年（昭和73年）", 1998},
		{"設立年不明", 0},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, extractYear(tt.input), "input: %s", tt.input)
	}
}

func TestExtractFloat(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"32.5歳", 32.5},
		{"40.0%", 40.0},
		{"なし", 0},
	}
	for _, tt := range tests {
		assert.InDelta(t, tt.want, extractFloat(tt.input), 0.001, "input: %s", tt.input)
	}
}
