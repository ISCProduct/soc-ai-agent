package services

import (
	"Backend/domain/entity"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

type matchReasonTemplate struct {
	Intro   string `json:"intro"`
	Fit     string `json:"fit"`
	User    string `json:"user"`
	Company string `json:"company"`
	Closing string `json:"closing"`
}

type matchReasonTemplates struct {
	Default matchReasonTemplate `json:"default"`
}

var (
	templatesOnce sync.Once
	templates     matchReasonTemplates
)

func loadMatchReasonTemplates() matchReasonTemplates {
	templatesOnce.Do(func() {
		templates = matchReasonTemplates{
			Default: matchReasonTemplate{
				Intro:   "総合マッチ度{{match_score}}%。{{company_name}}（{{industry}}）は、あなたの志向と企業が重視する人物像が高い水準で一致しています。",
				Fit:     "特に{{top_matches}}の一致度が高く、この企業の仕事の進め方や価値観と噛み合っています。",
				User:    "これまでの回答からは、{{user_strengths}}が強みとして読み取れます。強みが発揮できる場面が多く、成長の機会が得られる見込みがあります。",
				Company: "{{company_context}}",
				Closing: "上記の理由から、{{company_name}}は「無理なく力を発揮しつつ、次の成長につなげられる」候補として特におすすめです。",
			},
		}

		path := "config/match_reason_templates.json"
		if _, err := os.Stat(path); err != nil {
			return
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var fileTemplates matchReasonTemplates
		if err := json.Unmarshal(raw, &fileTemplates); err != nil {
			return
		}
		if fileTemplates.Default.Intro != "" {
			templates = fileTemplates
		}
	})

	return templates
}

type scoreItem struct {
	label string
	score float64
}

func BuildMatchReason(match *entity.UserCompanyMatch, userScores []entity.UserWeightScore) string {
	if match == nil || match.Company == nil || match.Company.ID == 0 {
		return ""
	}
	if match.MatchReason != "" {
		return match.MatchReason
	}

	tpl := loadMatchReasonTemplates().Default

	topMatches := topMatchSummaries(match)
	userStrengths := topUserStrengths(userScores)
	companyContext := buildCompanyContext(match.Company)

	replacer := strings.NewReplacer(
		"{{match_score}}", fmt.Sprintf("%.0f", match.MatchScore),
		"{{company_name}}", match.Company.Name,
		"{{industry}}", fallbackText(match.Company.Industry, "IT業界"),
		"{{top_matches}}", topMatches,
		"{{user_strengths}}", userStrengths,
		"{{company_context}}", companyContext,
	)

	sections := []string{
		replacer.Replace(tpl.Intro),
		replacer.Replace(tpl.Fit),
		replacer.Replace(tpl.User),
		replacer.Replace(tpl.Company),
		replacer.Replace(tpl.Closing),
	}

	return strings.Join(filterNonEmpty(sections), "\n\n")
}

func topMatchSummaries(match *entity.UserCompanyMatch) string {
	scores := []scoreItem{
		{label: "技術志向", score: match.TechnicalMatch},
		{label: "チームワーク", score: match.TeamworkMatch},
		{label: "リーダーシップ", score: match.LeadershipMatch},
		{label: "創造性", score: match.CreativityMatch},
		{label: "安定志向", score: match.StabilityMatch},
		{label: "成長志向", score: match.GrowthMatch},
		{label: "ワークライフバランス", score: match.WorkLifeMatch},
		{label: "チャレンジ志向", score: match.ChallengeMatch},
		{label: "細部志向", score: match.DetailMatch},
		{label: "コミュニケーション力", score: match.CommunicationMatch},
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	parts := []string{}
	for i := 0; i < len(scores) && i < 3; i++ {
		parts = append(parts, fmt.Sprintf("%s(%.0f%%)", scores[i].label, scores[i].score))
	}
	if len(parts) == 0 {
		return "複数の評価軸"
	}
	return strings.Join(parts, "・")
}

func topUserStrengths(scores []entity.UserWeightScore) string {
	if len(scores) == 0 {
		return "複数の軸でバランス良く評価"
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})
	parts := []string{}
	for i := 0; i < len(scores) && i < 3; i++ {
		if scores[i].Score <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s(%d点)", scores[i].WeightCategory, scores[i].Score))
	}
	if len(parts) == 0 {
		return "幅広い観点"
	}
	return strings.Join(parts, "・")
}

func buildCompanyContext(company *entity.Company) string {
	if company == nil {
		return "事業内容や開発環境に合った適性が活かせる企業です。"
	}
	// DB に載っている事実のみ（Search / 都度 LLM 調査はしない）#588
	parts := []string{}
	if strings.TrimSpace(company.MainBusiness) != "" {
		parts = append(parts, fmt.Sprintf("主な事業は「%s」です。", trimReasonText(company.MainBusiness, 120)))
	} else if strings.TrimSpace(company.Description) != "" {
		parts = append(parts, fmt.Sprintf("概要として「%s」が登録されています。", trimReasonText(company.Description, 120)))
	}
	if strings.TrimSpace(company.Culture) != "" {
		parts = append(parts, fmt.Sprintf("企業文化として「%s」が特徴です。", trimReasonText(company.Culture, 80)))
	}
	if strings.TrimSpace(company.WorkStyle) != "" {
		parts = append(parts, fmt.Sprintf("働き方は「%s」です。", company.WorkStyle))
	}
	if strings.TrimSpace(company.TechStack) != "" && company.TechStack != "null" && company.TechStack != "[]" {
		parts = append(parts, fmt.Sprintf("技術スタックには「%s」などがあります。", trimReasonText(company.TechStack, 80)))
	}
	if len(parts) == 0 {
		return "登録済みの企業プロファイルに基づき、志向の一致を重視した候補です。"
	}
	return strings.Join(parts, "")
}

func trimReasonText(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if max <= 0 || len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func filterNonEmpty(parts []string) []string {
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			result = append(result, p)
		}
	}
	return result
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func parseTechStack(techStack string) []string {
	if techStack == "" {
		return []string{}
	}
	var stack []string
	if err := json.Unmarshal([]byte(techStack), &stack); err == nil {
		return stack
	}
	parts := strings.Split(techStack, ",")
	result := []string{}
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
