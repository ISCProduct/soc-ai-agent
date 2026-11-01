package company

import (
	"reflect"
	"testing"
	"time"
)

func TestPlanFix(t *testing.T) {
	future := time.Now().Year() + 5
	tests := []struct {
		name         string
		row          AuditRow
		wantOK       bool
		wantChanges  map[string]any
		wantUnmapped map[string]string
	}{
		{"正規値は何も変えない", AuditRow{Industry: "IT・ソフトウェア", WorkStyle: "リモート", FoundedYear: 2000, EmployeeCount: 100, EmployeeCountBasis: "standalone", WebsiteURL: "https://a.example"}, false, map[string]any{}, map[string]string{}},
		{"全て空でも何も変えない", AuditRow{}, false, map[string]any{}, map[string]string{}},
		{"業種の表記ゆれを寄せる", AuditRow{Industry: "情報・通信業"}, true, map[string]any{"industry": "IT・ソフトウェア"}, map[string]string{}},
		{"寄せられない業種は消さず報告する", AuditRow{Industry: "旅行業"}, true, map[string]any{}, map[string]string{"industry": "旅行業"}},
		{"勤務スタイルを寄せる", AuditRow{WorkStyle: "週3リモート週2出社"}, true, map[string]any{"work_style": "ハイブリッド"}, map[string]string{}},
		{"寄せられない勤務スタイルは消さず報告する", AuditRow{WorkStyle: "不明"}, true, map[string]any{}, map[string]string{"work_style": "不明"}},
		{"範囲外の設立年は0にする", AuditRow{FoundedYear: 1500}, true, map[string]any{"founded_year": 0}, map[string]string{}},
		{"未来すぎる設立年は0にする", AuditRow{FoundedYear: future}, true, map[string]any{"founded_year": 0}, map[string]string{}},
		{"範囲外の従業員数は0にして基準も消す", AuditRow{EmployeeCount: 9_000_000, EmployeeCountBasis: "consolidated"}, true, map[string]any{"employee_count": 0, "employee_count_basis": ""}, map[string]string{}},
		{"従業員数0に基準だけ残っていれば消す", AuditRow{EmployeeCountBasis: "standalone"}, true, map[string]any{"employee_count_basis": ""}, map[string]string{}},
		{"URLがhttpで始まらない値は消さず報告する", AuditRow{WebsiteURL: "www.example.co.jp"}, true, map[string]any{}, map[string]string{"website_url": "www.example.co.jp"}},
		{"大文字スキームのURLは消さない", AuditRow{WebsiteURL: "HTTPS://Example.com"}, false, map[string]any{}, map[string]string{}},
		{"URLの前後の空白だけ落とす", AuditRow{WebsiteURL: " https://a.example "}, true, map[string]any{"website_url": "https://a.example"}, map[string]string{}},
		{"否定を含む勤務スタイルは寄せず報告する(出社不要)", AuditRow{WorkStyle: "出社不要"}, true, map[string]any{}, map[string]string{"work_style": "出社不要"}},
		{"否定を含む勤務スタイルは寄せず報告する(リモート不可)", AuditRow{WorkStyle: "リモート不可・原則出社"}, true, map[string]any{}, map[string]string{"work_style": "リモート不可・原則出社"}},
		{"空白だけの業種は空へ直す", AuditRow{Industry: "  "}, true, map[string]any{"industry": ""}, map[string]string{}},
		{"前後に空白のある正規の業種は空白を落とす", AuditRow{Industry: "金融・保険 "}, true, map[string]any{"industry": "金融・保険"}, map[string]string{}},
		{"設立年の境界(1800は許す)", AuditRow{FoundedYear: 1800}, false, map[string]any{}, map[string]string{}},
		{"設立年の境界(1799は0)", AuditRow{FoundedYear: 1799}, true, map[string]any{"founded_year": 0}, map[string]string{}},
		{"従業員数の境界(3,000,000は許す)", AuditRow{EmployeeCount: 3_000_000, EmployeeCountBasis: "consolidated"}, false, map[string]any{}, map[string]string{}},
		{"従業員数の境界(3,000,001は0)", AuditRow{EmployeeCount: 3_000_001, EmployeeCountBasis: "consolidated"}, true, map[string]any{"employee_count": 0, "employee_count_basis": ""}, map[string]string{}},
		{"変更と報告が同時に出る", AuditRow{Industry: "情報通信業", WorkStyle: "不明"}, true, map[string]any{"industry": "IT・ソフトウェア"}, map[string]string{"work_style": "不明"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, ok := PlanFix(tt.row)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (plan=%+v)", ok, tt.wantOK, plan)
			}
			if !reflect.DeepEqual(plan.Changes, tt.wantChanges) {
				t.Errorf("Changes = %v, want %v", plan.Changes, tt.wantChanges)
			}
			if !reflect.DeepEqual(plan.Unmapped, tt.wantUnmapped) {
				t.Errorf("Unmapped = %v, want %v", plan.Unmapped, tt.wantUnmapped)
			}
		})
	}
}

func TestDistribution(t *testing.T) {
	got := Distribution([]string{"b", "a", "b", "", "a", "c", ""})
	// 値は変換しない（空文字は空文字のまま数える）。同数は値の昇順で、空文字が先頭。
	want := []ValueCount{{"", 2}, {"a", 2}, {"b", 2}, {"c", 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// sanitizeCompanyInfo は人数が 0 か範囲外のときだけ基準を空にし、基準の値そのものは見ない。
// そのため人数が有効で基準が選択肢外(例: "unknown")の行は、変更も報告もされず点検から漏れていた。
func TestPlanFixReportsUnknownEmployeeCountBasis(t *testing.T) {
	tests := []struct {
		name         string
		count        int
		basis        string
		wantUnmapped string // "" なら Unmapped に出ないことを期待
		wantChanged  bool
	}{
		{"選択肢外の基準は要判断へ", 100, "unknown", "unknown", false},
		{"全角や自由記述も要判断へ", 100, "連結", "連結", false},
		{"consolidated はそのまま", 100, "consolidated", "", false},
		{"standalone はそのまま", 100, "standalone", "", false},
		{"空(不明)はそのまま", 100, "", "", false},
		// 人数が 0 のときは sanitize が基準を空にするので、要判断ではなく修正になる。
		{"人数0なら基準を空にする修正", 0, "unknown", "", true},
		{"人数が範囲外なら基準を空にする修正", -5, "unknown", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, _ := PlanFix(AuditRow{ID: 1, Name: "A社", EmployeeCount: tt.count, EmployeeCountBasis: tt.basis})
			got, inUnmapped := plan.Unmapped["employee_count_basis"]
			if tt.wantUnmapped == "" {
				if inUnmapped {
					t.Errorf("要判断に出した: %q", got)
				}
			} else if got != tt.wantUnmapped {
				t.Errorf("要判断の値 = %q(あり=%v), want %q", got, inUnmapped, tt.wantUnmapped)
			}
			if _, changed := plan.Changes["employee_count_basis"]; changed != tt.wantChanged {
				t.Errorf("修正対象 = %v, want %v", changed, tt.wantChanged)
			}
		})
	}
}

// 空文字を "(空)" へ置き換えてから数えると、実データの "(空)" と合算されて
// 分布が読めなくなる(CLI は合算結果を「正規」と表示していた)。
func TestDistributionKeepsEmptyAndLiteralSeparate(t *testing.T) {
	got := Distribution([]string{"", "(空)", "", "IT・ソフトウェア"})
	counts := map[string]int{}
	for _, d := range got {
		counts[d.Value] = d.Count
	}
	if counts[""] != 2 {
		t.Errorf("空文字の件数 = %d, want 2", counts[""])
	}
	if counts["(空)"] != 1 {
		t.Errorf("実データ \"(空)\" の件数 = %d, want 1", counts["(空)"])
	}
	if len(got) != 3 {
		t.Errorf("行数 = %d, want 3（空文字 / (空) / IT・ソフトウェア）", len(got))
	}
}

// 表示だけ "(空)" に寄せる。集計は元の値のままなので、実データの "(空)" は別行に残る。
func TestDisplayValue(t *testing.T) {
	if got := DisplayValue(""); got != "(空)" {
		t.Errorf("DisplayValue(\"\") = %q, want \"(空)\"", got)
	}
	if got := DisplayValue("(空)"); got != "(空)" {
		t.Errorf("DisplayValue(\"(空)\") = %q", got)
	}
	if got := DisplayValue("IT・ソフトウェア"); got != "IT・ソフトウェア" {
		t.Errorf("DisplayValue = %q", got)
	}
}
