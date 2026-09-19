package company

// スキーマと正規化のテスト。
// 実行: cd Backend && go test ./internal/services/company/ -run "TestCompanyInfoResponseSchema|TestNormalizeIndustry|TestNormalizeWorkStyle|TestSanitizeCompanyInfo" -v

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCompanyInfoResponseSchema(t *testing.T) {
	schema := companyInfoResponseSchema()
	if schema == nil {
		t.Fatal("スキーマが nil")
	}

	var doc map[string]any
	if err := json.Unmarshal(schema.Schema, &doc); err != nil {
		t.Fatalf("スキーマが JSON として壊れている: %v", err)
	}

	// strict:true では additionalProperties:false と全プロパティの required が必須。
	// これが欠けると API がスキーマごと拒否する。
	if doc["additionalProperties"] != false {
		t.Error("additionalProperties は false でなければ strict が通らない")
	}
	props, _ := doc["properties"].(map[string]any)
	required, _ := doc["required"].([]any)
	if len(props) != len(required) {
		t.Errorf("required がプロパティ数と一致しない: props=%d required=%d", len(props), len(required))
	}

	// 業種と勤務スタイルは列挙で縛る。ここが自由記述だと表記が揺れる。
	industry, _ := props["industry"].(map[string]any)
	if _, ok := industry["enum"]; !ok {
		t.Error("industry は enum で縛るべき")
	}
	// work_style は enum にしない。実測で enum の並び順によって分類が変わり、
	// 「原則出社」が先頭の値になってしまった。分類は normalizeWorkStyle で行う。
	workStyle, _ := props["work_style"].(map[string]any)
	if _, ok := workStyle["enum"]; ok {
		t.Error("work_style は enum にしない（順序バイアスで分類がぶれる）")
	}
}

func TestNormalizeIndustry(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// 実測で同じ会社に対して両方が出た。揃わないとマッチングの結果が変わる。
		{"IT・ソフトウェア", "IT・ソフトウェア"},
		{"情報・通信業", "IT・ソフトウェア"},
		{"情報通信業", "IT・ソフトウェア"},
		{"IT業界", "IT・ソフトウェア"},
		{"SaaS", "IT・ソフトウェア"},
		{"自動車産業", "製造業"},
		{"メーカー", "製造業"},
		{"金融", "金融・保険"},
		{"不動産", "建設・不動産"},
		{"コンサル", "コンサルティング"},
		{"IT・ソフトウェア開発", "IT・ソフトウェア"}, // 選択肢を含む表記
		{"その他", "その他"},
		{"", ""},
		// 寄せられないものは空にする。誤った分類を入れるより害が小さい。
		{"宇宙開発事業", ""},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeIndustry(tt.in); got != tt.want {
				t.Errorf("normalizeIndustry(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeWorkStyle(t *testing.T) {
	tests := []struct{ in, want string }{
		{"リモート", "リモート"},
		{"フルリモート", "リモート"},
		{"在宅勤務", "リモート"},
		{"ハイブリッド", "ハイブリッド"},
		{"リモートとオフィスの併用", "ハイブリッド"},
		{"リモート勤務可、週2出社", "ハイブリッド"}, // リモートだけ見ると取り違える
		{"オフィス", "オフィス"},
		{"原則出社", "オフィス"},
		{"週3リモート週2出社", "ハイブリッド"}, // 実APIでモデルが取り違えたケース
		{"テレワーク中心", "リモート"},
		{"対面での協業を重視", "オフィス"},
		{"", ""},
		{"フレックス", ""}, // 勤務時間の話で場所ではない
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeWorkStyle(tt.in); got != tt.want {
				t.Errorf("normalizeWorkStyle(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeCompanyInfo(t *testing.T) {
	nextYear := time.Now().Year() + 1

	tests := []struct {
		name   string
		in     CompanyInfoResult
		verify func(*testing.T, *CompanyInfoResult)
	}{
		{
			name: "設立年が古すぎれば不明にする",
			in:   CompanyInfoResult{FoundedYear: 1200},
			verify: func(t *testing.T, r *CompanyInfoResult) {
				if r.FoundedYear != 0 {
					t.Errorf("FoundedYear=%d, want 0", r.FoundedYear)
				}
			},
		},
		{
			name: "設立年が未来すぎれば不明にする",
			in:   CompanyInfoResult{FoundedYear: nextYear + 5},
			verify: func(t *testing.T, r *CompanyInfoResult) {
				if r.FoundedYear != 0 {
					t.Errorf("FoundedYear=%d, want 0", r.FoundedYear)
				}
			},
		},
		{
			name: "妥当な設立年は残す",
			in:   CompanyInfoResult{FoundedYear: 1937},
			verify: func(t *testing.T, r *CompanyInfoResult) {
				if r.FoundedYear != 1937 {
					t.Errorf("FoundedYear=%d, want 1937", r.FoundedYear)
				}
			},
		},
		{
			name: "従業員数が非現実的なら不明にする",
			in:   CompanyInfoResult{EmployeeCount: 99_000_000, EmployeeCountBasis: "consolidated"},
			verify: func(t *testing.T, r *CompanyInfoResult) {
				if r.EmployeeCount != 0 || r.EmployeeCountBasis != "" {
					t.Errorf("EmployeeCount=%d basis=%q, want 0/空", r.EmployeeCount, r.EmployeeCountBasis)
				}
			},
		},
		{
			name: "従業員数0なら基準も空にする",
			in:   CompanyInfoResult{EmployeeCount: 0, EmployeeCountBasis: "consolidated"},
			verify: func(t *testing.T, r *CompanyInfoResult) {
				if r.EmployeeCountBasis != "" {
					t.Errorf("basis=%q, want 空", r.EmployeeCountBasis)
				}
			},
		},
		{
			name: "http以外のURLは落とす",
			in:   CompanyInfoResult{WebsiteURL: "不明"},
			verify: func(t *testing.T, r *CompanyInfoResult) {
				if r.WebsiteURL != "" {
					t.Errorf("WebsiteURL=%q, want 空", r.WebsiteURL)
				}
			},
		},
		{
			name: "httpsのURLは残す",
			in:   CompanyInfoResult{WebsiteURL: "https://example.co.jp/"},
			verify: func(t *testing.T, r *CompanyInfoResult) {
				if r.WebsiteURL != "https://example.co.jp/" {
					t.Errorf("WebsiteURL=%q が落ちた", r.WebsiteURL)
				}
			},
		},
		{
			name: "業種と勤務スタイルを正規化する",
			in:   CompanyInfoResult{Industry: "情報・通信業", WorkStyle: "フルリモート"},
			verify: func(t *testing.T, r *CompanyInfoResult) {
				if r.Industry != "IT・ソフトウェア" {
					t.Errorf("Industry=%q", r.Industry)
				}
				if r.WorkStyle != "リモート" {
					t.Errorf("WorkStyle=%q", r.WorkStyle)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.in
			sanitizeCompanyInfo(&r)
			tt.verify(t, &r)
		})
	}

	t.Run("nilでも落ちない", func(t *testing.T) {
		sanitizeCompanyInfo(nil)
	})
}

// パース経路が必ず正規化を通ることを固定する。
// ここを外すと、スキーマ非対応モデルへ落ちたときに生の値が DB へ入る。
func TestParseCompanyInfoResult_正規化を通る(t *testing.T) {
	raw := `{"description":"概要","industry":"情報・通信業","location":"東京都港区",
	         "website_url":"不明","founded_year":1200,"employee_count":99000000,
	         "employee_count_basis":"consolidated","main_business":"事業",
	         "culture":"文化","work_style":"フルリモート","tech_stack":"","welfare_details":""}`

	got, err := parseCompanyInfoResult(raw)
	if err != nil {
		t.Fatalf("パースに失敗: %v", err)
	}
	for _, c := range []struct {
		field, got, want string
	}{
		{"Industry", got.Industry, "IT・ソフトウェア"},
		{"WorkStyle", got.WorkStyle, "リモート"},
		{"WebsiteURL", got.WebsiteURL, ""},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.field, c.got, c.want)
		}
	}
	if got.FoundedYear != 0 {
		t.Errorf("FoundedYear = %d, want 0", got.FoundedYear)
	}
	if got.EmployeeCount != 0 {
		t.Errorf("EmployeeCount = %d, want 0", got.EmployeeCount)
	}
}

// 選択肢に重複や空白が無いこと（enum に混ざると分類が不安定になる）。
func TestIndustryOptions健全性(t *testing.T) {
	seen := make(map[string]bool, len(IndustryOptions))
	for _, opt := range IndustryOptions {
		if strings.TrimSpace(opt) != opt || opt == "" {
			t.Errorf("前後に空白、または空の選択肢: %q", opt)
		}
		if seen[opt] {
			t.Errorf("選択肢が重複: %q", opt)
		}
		seen[opt] = true
	}
	if !seen["その他"] {
		t.Error("該当が無い場合の受け皿として「その他」が必要")
	}
}
