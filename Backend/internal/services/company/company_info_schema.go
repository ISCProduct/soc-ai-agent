package company

// 企業情報のJSON Schema（Structured Outputs用）と、受け取った値の正規化。
//
// これまでは JSON mode しか使っておらず、「有効なJSONであること」以上の保証が無かった。
// キーの欠落・型違い・想定外の業種名がそのまま DB に入り、同じ会社でも取得経路によって
// 業種が「IT・ソフトウェア」だったり「情報・通信業」だったりする状態になっていた。
// Industry は companies テーブルでインデックス対象かつマッチングにも使われるため、
// 表記が揺れると検索結果とマッチ結果が変わる。
//
// 二段構えで塞ぐ。
//   1. スキーマ(strict)で型と列挙値をモデル側に守らせる
//   2. それでも来た想定外の値を、受け取り側で正規化・範囲チェックする
//
// 2 を省かないのは、スキーマ非対応のモデルへフォールバックした場合と、
// 既存データを後から通す場合に備えるため。

import (
	"encoding/json"
	"strings"
	"time"

	appopenai "Backend/internal/openai"
)

// IndustryOptions は業種の選択肢。自由記述をやめて、この中から選ばせる。
//
// 日本標準産業分類そのままでは求人の文脈に合わないため、学生が企業を探すときの
// 粒度に寄せている。該当が無い場合のために「その他」を必ず残す。
var IndustryOptions = []string{
	"IT・ソフトウェア",
	"通信",
	"金融・保険",
	"製造業",
	"商社",
	"小売",
	"医療・福祉",
	"教育",
	"建設・不動産",
	"運輸・物流",
	"広告・メディア",
	"コンサルティング",
	"人材サービス",
	"飲食・宿泊",
	"エネルギー・インフラ",
	"公務・団体",
	"その他",
}

// WorkStyleOptions は勤務スタイルの最終的な値。DBに入るのはこのいずれか。
//
// モデルにこの中から選ばせる形はやめた。実測では enum の並び順で答えが変わり、
// 「原則出社」に対して先頭の値を返してしまう。順序を入れ替えると結果も変わった。
// enum は形式を保証するだけで、分類の正しさまでは保証しない。
//
// そこでモデルには勤務形態を説明する原文をそのまま返させ、分類は
// normalizeWorkStyle で行う。分類規則がコード側にあればテストで固定できる。
var WorkStyleOptions = []string{"リモート", "ハイブリッド", "オフィス", ""}

// industryAliases は表記ゆれを選択肢へ寄せるための対応表。
//
// 実測で「IT・ソフトウェア」と「情報・通信業」が同じ会社に対して出た。
// スキーマで縛ってもフォールバック時や既存データには効かないので、ここで吸収する。
var industryAliases = map[string]string{
	"情報・通信業":    "IT・ソフトウェア",
	"情報通信業":     "IT・ソフトウェア",
	"情報通信":      "IT・ソフトウェア",
	"it":        "IT・ソフトウェア",
	"it業界":      "IT・ソフトウェア",
	"itサービス":    "IT・ソフトウェア",
	"ソフトウェア":    "IT・ソフトウェア",
	"インターネット":   "IT・ソフトウェア",
	"web":       "IT・ソフトウェア",
	"saas":      "IT・ソフトウェア",
	"金融":        "金融・保険",
	"保険":        "金融・保険",
	"銀行":        "金融・保険",
	"証券":        "金融・保険",
	"メーカー":      "製造業",
	"製造":        "製造業",
	"自動車産業":     "製造業",
	"自動車":       "製造業",
	"流通":        "小売",
	"小売業":       "小売",
	"不動産":       "建設・不動産",
	"建設":        "建設・不動産",
	"物流":        "運輸・物流",
	"運輸":        "運輸・物流",
	"広告":        "広告・メディア",
	"メディア":      "広告・メディア",
	"出版":        "広告・メディア",
	"コンサル":      "コンサルティング",
	"人材":        "人材サービス",
	"教育・学習支援業":  "教育",
	"医療":        "医療・福祉",
	"福祉":        "医療・福祉",
	"エネルギー":     "エネルギー・インフラ",
	"電気・ガス・水道業": "エネルギー・インフラ",
	"公務":        "公務・団体",
}

// companyInfoResponseSchema は Structured Outputs へ渡すスキーマ。
//
// strict:true では additionalProperties:false と、全プロパティの required が要る。
// 「不明」を表すために型で null を許すのではなく、空文字 / 0 を使う（既存の扱いに合わせる）。
func companyInfoResponseSchema() *appopenai.ResponseSchema {
	raw, err := json.Marshal(map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"description", "industry", "location", "website_url", "founded_year",
			"employee_count", "employee_count_basis", "main_business", "culture",
			"work_style", "tech_stack", "welfare_details",
		},
		"properties": map[string]any{
			"description":          map[string]any{"type": "string", "description": "企業概要（100〜200文字程度）。不明なら空文字"},
			"industry":             map[string]any{"type": "string", "enum": IndustryOptions, "description": "最も近いものを1つ。該当が無ければ その他"},
			"location":             map[string]any{"type": "string", "description": "本社所在地（例: 東京都渋谷区）。不明なら空文字"},
			"website_url":          map[string]any{"type": "string", "description": "公式サイトURL（https://から始まる）。不明なら空文字"},
			"founded_year":         map[string]any{"type": "integer", "description": "設立年。不明なら0"},
			"employee_count":       map[string]any{"type": "integer", "description": "従業員数。連結を優先。不明なら0"},
			"employee_count_basis": map[string]any{"type": "string", "enum": []string{"consolidated", "standalone", ""}, "description": "従業員数の基準。不明なら空文字"},
			"main_business":        map[string]any{"type": "string", "description": "主要事業内容（50〜100文字程度）。不明なら空文字"},
			"culture":              map[string]any{"type": "string", "description": "企業文化・働き方の特徴（50〜100文字程度）。不明なら空文字"},
			// 分類せず、働く場所についての記述をそのまま返させる。分類は normalizeWorkStyle が行う。
			"work_style":      map[string]any{"type": "string", "description": "働く場所についての記述をそのまま書く（例: 週3リモート週2出社、原則出社、フルリモート）。記載が無ければ空文字。推測しない"},
			"tech_stack":      map[string]any{"type": "string", "description": "主要技術スタック（カンマ区切り）。不明なら空文字"},
			"welfare_details": map[string]any{"type": "string", "description": "福利厚生の要点。不明なら空文字"},
		},
	})
	if err != nil {
		// スキーマはリテラルから組むので実行時には失敗しない。
		// 万一失敗したらスキーマ無し（JSON mode）で続行する。
		return nil
	}
	return &appopenai.ResponseSchema{Name: "company_info", Schema: raw}
}

// normalizeIndustry は業種を選択肢のいずれかへ寄せる。
// 寄せられない場合は空文字を返す（誤った分類を入れるより空のほうが害が小さい）。
func normalizeIndustry(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	for _, opt := range IndustryOptions {
		if v == opt {
			return opt
		}
	}

	key := strings.ToLower(strings.TrimSpace(v))
	key = strings.TrimSuffix(key, "業界")
	if mapped, ok := industryAliases[key]; ok {
		return mapped
	}
	// 「IT・ソフトウェア開発」のように選択肢を含む表記を拾う。
	for _, opt := range IndustryOptions {
		if opt != "その他" && strings.Contains(v, opt) {
			return opt
		}
	}
	return ""
}

// normalizeWorkStyle は勤務スタイルを選択肢へ寄せる。
// リモート要素と出社要素の有無で決める。
// 「原則出社」をハイブリッドにしてしまう取り違えを避けるため、
// 出社だけの表現とリモートを含む表現を分けて数える。
func normalizeWorkStyle(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}

	hasRemote := containsAny(v, "リモート", "在宅", "テレワーク")
	hasOffice := containsAny(v, "オフィス", "出社", "常駐", "対面")
	hasHybrid := containsAny(v, "ハイブリッド", "併用")

	switch {
	case hasHybrid, hasRemote && hasOffice:
		return "ハイブリッド"
	case hasRemote:
		return "リモート"
	case hasOffice:
		return "オフィス"
	}
	return ""
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// 年と人数の許容範囲。明らかに外れた値は「不明」として扱う。
const (
	minFoundedYear      = 1800
	maxEmployeeCount    = 3_000_000 // 世界最大級の企業でも200万人台
	maxFoundedYearAhead = 1         // 設立予定の企業を考慮して翌年まで許す
)

// sanitizeCompanyInfo は受け取った値を正規化し、範囲外を落とす。
//
// スキーマで縛れない内容（年や人数の妥当性、業種の表記ゆれ）はここで潰す。
// 誤った値を入れるより、空にして「不明」と分かるほうが後の判断を誤らせない。
func sanitizeCompanyInfo(r *CompanyInfoResult) {
	if r == nil {
		return
	}

	r.Industry = normalizeIndustry(r.Industry)
	r.WorkStyle = normalizeWorkStyle(r.WorkStyle)

	maxYear := time.Now().Year() + maxFoundedYearAhead
	if r.FoundedYear != 0 && (r.FoundedYear < minFoundedYear || r.FoundedYear > maxYear) {
		r.FoundedYear = 0
	}
	if r.EmployeeCount < 0 || r.EmployeeCount > maxEmployeeCount {
		r.EmployeeCount = 0
		r.EmployeeCountBasis = ""
	}
	if r.EmployeeCount == 0 {
		r.EmployeeCountBasis = ""
	}

	// URLはhttp(s)のみ受ける。相対パスや「不明」といった文字列が入ることがある。
	if u := strings.TrimSpace(r.WebsiteURL); u != "" &&
		!strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		r.WebsiteURL = ""
	}
}
