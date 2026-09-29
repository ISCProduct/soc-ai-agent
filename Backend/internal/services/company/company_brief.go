package company

import (
	"Backend/internal/models"
	"fmt"
	"strings"
)

// BuildCompanyBrief は共有キャッシュ（companies + weight profile）から
// 壁打ち/面接/レビュー用の短いスナップショット文を組み立てる。
// Search / LLM 調査は行わない。
func BuildCompanyBrief(company *models.Company, profile *models.CompanyWeightProfile) string {
	if company == nil || !briefVisible(company) {
		return ""
	}
	var b strings.Builder
	name := strings.TrimSpace(company.Name)
	if name != "" {
		fmt.Fprintf(&b, "企業名: %s\n", name)
	}
	if industry := strings.TrimSpace(company.Industry); industry != "" {
		fmt.Fprintf(&b, "業種: %s\n", industry)
	}
	if loc := strings.TrimSpace(company.Location); loc != "" {
		fmt.Fprintf(&b, "所在地: %s\n", loc)
	}
	business := strings.TrimSpace(company.MainBusiness)
	if business == "" {
		business = strings.TrimSpace(company.Description)
	}
	if business != "" {
		fmt.Fprintf(&b, "事業: %s\n", trimRunes(business, 200))
	}
	if culture := strings.TrimSpace(company.Culture); culture != "" {
		fmt.Fprintf(&b, "文化: %s\n", trimRunes(culture, 120))
	}
	if work := strings.TrimSpace(company.WorkStyle); work != "" {
		fmt.Fprintf(&b, "働き方: %s\n", work)
	}
	if tech := strings.TrimSpace(company.TechStack); tech != "" && tech != "null" && tech != "[]" {
		fmt.Fprintf(&b, "技術スタック: %s\n", trimRunes(tech, 120))
	}
	if profile != nil {
		top := topWeightLabels(profile, 3)
		if len(top) > 0 {
			fmt.Fprintf(&b, "重視傾向: %s\n", strings.Join(top, "、"))
		}
	}
	return strings.TrimSpace(b.String())
}

// briefVisible は brief に文面を出してよい企業行かを判定する（#1600）。
//
// /company-entry は無認証（honeypot とレート制限のみ）で投稿でき、
// data_status='draft' / is_provisional=true / is_guest_entry=true の企業行が
// 即座に出来上がる（company_entry_service.go）。審査前のその文面が
// 面接・履歴書レビューのプロンプトへ入ると、任意の指示文を仕込めてしまう。
//
// 条件は SQL 側のガード guestEntryVisibilityGuard（company_query_repository.go）の
// 企業行側の半分と同じ。二重に持つ理由は、brief の読み出し口が
// shared.CompanyBriefReader インターフェース越しで、フィルタ無しの
// CompanyRepository を注入しても型が通るため。現在の DI（cmd/server/main.go）は
// CompanyPublicRepository を渡しているが、その取り違えは型では防げない。
//
// data_status='published' は必須にしない。draft は自動収集した全企業の既定状態でもあり
// （実データで 842社中 752社）、published だけに絞ると面接の企業選択・履歴書レビューの
// 企業ブリーフがほぼ空になる（guestEntryVisibilityGuard のコメント参照）。
// ゲスト投稿由来かどうかで分けるのが #1203 / #1409 で決めた線。
func briefVisible(c *models.Company) bool {
	if !c.IsGuestEntry {
		return true
	}
	// 管理者が公開したものは出す。公開後に却下されると is_active=false になる。
	return c.DataStatus == "published" && c.IsActive
}

func trimRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func topWeightLabels(profile *models.CompanyWeightProfile, n int) []string {
	type item struct {
		label string
		score int
	}
	items := []item{
		{"技術志向", profile.TechnicalOrientation},
		{"チームワーク", profile.TeamworkOrientation},
		{"リーダーシップ", profile.LeadershipOrientation},
		{"創造性", profile.CreativityOrientation},
		{"安定志向", profile.StabilityOrientation},
		{"成長志向", profile.GrowthOrientation},
		{"ワークライフバランス", profile.WorkLifeBalance},
		{"チャレンジ", profile.ChallengeSeeking},
		{"細部志向", profile.DetailOrientation},
		{"コミュニケーション", profile.CommunicationSkill},
	}
	// 単純選択ソートで上位 n
	for i := 0; i < len(items); i++ {
		maxIdx := i
		for j := i + 1; j < len(items); j++ {
			if items[j].score > items[maxIdx].score {
				maxIdx = j
			}
		}
		items[i], items[maxIdx] = items[maxIdx], items[i]
	}
	out := make([]string, 0, n)
	for i := 0; i < len(items) && len(out) < n; i++ {
		if items[i].score <= 50 {
			continue
		}
		out = append(out, fmt.Sprintf("%s(%d)", items[i].label, items[i].score))
	}
	return out
}
