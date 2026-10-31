package company

// 既存の企業データを sanitizeCompanyInfo 相当の基準で点検し、直す差分を作る(SOCAIAGENT-326)。
//
// 取得時の正規化は新規取得分にしか効かないので、すでに DB にある値の表記ゆれは残る。
// ここは「何が変わるか」を決める純粋関数だけを持ち、DB の読み書きは cmd/company-audit が行う。
//
// 取得時と違い、寄せられない業種・勤務スタイルは空にせず、そのまま残して報告する。
// 取得時は「誤分類より空」だが、既存値を一括で消すと元に戻せない情報を失うため。

import (
	"sort"
	"strings"
)

// AuditRow は点検に必要な companies の列だけを持つ。
type AuditRow struct {
	ID                 uint
	Name               string
	Industry           string
	WorkStyle          string
	WebsiteURL         string
	FoundedYear        int
	EmployeeCount      int
	EmployeeCountBasis string
}

// FixPlan は1社ぶんの修正。Changes のキーは companies の列名。
type FixPlan struct {
	ID      uint
	Name    string
	Changes map[string]any
	// Unmapped は選択肢へ寄せられず、そのまま残した項目(列名 → 現在値)。
	Unmapped map[string]string
}

// PlanFix は row に対する修正を作る。変更も未対応も無ければ ok=false。
func PlanFix(row AuditRow) (plan FixPlan, ok bool) {
	plan = FixPlan{ID: row.ID, Name: row.Name, Changes: map[string]any{}, Unmapped: map[string]string{}}

	planChoice(&plan, "industry", row.Industry, normalizeIndustry)
	planChoice(&plan, "work_style", row.WorkStyle, normalizeAuditWorkStyle)

	// URL は自動で空にしない。前後の空白だけ落とし、http(s) で始まらない値は報告に回す。
	// 取得時の判定は小文字の https:// だけを通すため、既存の HTTPS:// や www. 始まりを消してしまう。
	if u := strings.TrimSpace(row.WebsiteURL); u != "" {
		if lu := strings.ToLower(u); !strings.HasPrefix(lu, "https://") && !strings.HasPrefix(lu, "http://") {
			plan.Unmapped["website_url"] = row.WebsiteURL
		} else if u != row.WebsiteURL {
			plan.Changes["website_url"] = u
		}
	} else if row.WebsiteURL != "" {
		plan.Changes["website_url"] = ""
	}

	// 年・人数は取得時と同じ範囲チェックを使う(明らかに誤りの値。適用前の値は backup に残る)。
	r := CompanyInfoResult{
		FoundedYear:        row.FoundedYear,
		EmployeeCount:      row.EmployeeCount,
		EmployeeCountBasis: row.EmployeeCountBasis,
	}
	sanitizeCompanyInfo(&r)
	if r.FoundedYear != row.FoundedYear {
		plan.Changes["founded_year"] = r.FoundedYear
	}
	if r.EmployeeCount != row.EmployeeCount {
		plan.Changes["employee_count"] = r.EmployeeCount
	}
	if r.EmployeeCountBasis != row.EmployeeCountBasis {
		plan.Changes["employee_count_basis"] = r.EmployeeCountBasis
	}

	return plan, len(plan.Changes) > 0 || len(plan.Unmapped) > 0
}

// ValueCount は値とその件数。
type ValueCount struct {
	Value string
	Count int
}

// Distribution は値ごとの件数を、多い順(同数は値の昇順)で返す。空文字は「(空)」に置き換える。
func Distribution(values []string) []ValueCount {
	m := map[string]int{}
	for _, v := range values {
		if v == "" {
			v = "(空)"
		}
		m[v]++
	}
	out := make([]ValueCount, 0, len(m))
	for v, c := range m {
		out = append(out, ValueCount{v, c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	return out
}

// IsIndustryOption は v が業種の選択肢そのものかを返す(分布の「正規/ゆれ」判定用)。
func IsIndustryOption(v string) bool {
	for _, o := range IndustryOptions {
		if v == o {
			return true
		}
	}
	return false
}

// planChoice は選択肢型の列(industry / work_style)について、寄せる・報告に回すを決める。
// 空白だけの値は空へ直す。寄せられない値は消さず Unmapped に残す。
func planChoice(plan *FixPlan, col, raw string, normalize func(string) string) {
	if raw == "" {
		return
	}
	v := strings.TrimSpace(raw)
	if v == "" {
		plan.Changes[col] = ""
		return
	}
	switch n := normalize(v); {
	case n == "":
		plan.Unmapped[col] = raw
	case n != raw:
		plan.Changes[col] = n
	}
}

// negations は勤務スタイルの否定表現。
var negations = []string{"不要", "不可", "なし", "ない", "禁止", "無し"}

// normalizeAuditWorkStyle は normalizeWorkStyle に、否定表現を除く条件を足したもの。
// 「出社不要」をオフィス、「リモート不可」をリモートに寄せるような取り違えは、
// 元の自由記述を失うので、否定を含む値は寄せずに要判断へ回す(空を返す)。
func normalizeAuditWorkStyle(raw string) string {
	if containsAny(raw, negations...) {
		return ""
	}
	return normalizeWorkStyle(raw)
}
