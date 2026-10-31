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
	want := []ValueCount{{"(空)", 2}, {"a", 2}, {"b", 2}, {"c", 1}}
	// 同数は値の昇順。"(空)" は "a" より前(文字コード順)。
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
