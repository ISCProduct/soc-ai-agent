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
	if company == nil {
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
