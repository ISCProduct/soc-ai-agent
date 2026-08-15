package analysis

import (
	"Backend/domain/entity"
	"fmt"
	"sort"
	"strings"
)

func (s *AnalysisScoringService) buildRecommendations(userID uint, sessionID string) AnalysisRecommendations {
	recommendations := AnalysisRecommendations{}

	topCategories, err := s.userWeightScoreRepo.FindTopCategories(userID, sessionID, 3)
	if err == nil {
		for _, score := range topCategories {
			recommendations.TopCategories = append(recommendations.TopCategories, CategoryRecommendation{
				Category: score.WeightCategory,
				Score:    score.Score,
			})
		}
	}

	if s.matchRepo == nil {
		return recommendations
	}

	topMatches, err := s.matchRepo.FindTopMatchesByUserAndSession(userID, sessionID, 3)
	if err != nil {
		return recommendations
	}

	for _, match := range topMatches {
		if match.Company == nil || match.Company.ID == 0 {
			continue
		}
		recommendations.TopCompanies = append(recommendations.TopCompanies, CompanyRecommendation{
			ID:    match.Company.ID,
			Name:  match.Company.Name,
			Score: match.MatchScore,
		})
	}
	return recommendations
}

func buildJobSuitabilityComment(scores []entity.UserWeightScore) (string, []JobSuitabilityRole) {
	if len(scores) == 0 {
		return "", nil
	}

	scoreMap := make(map[string]int)
	for _, s := range scores {
		scoreMap[s.WeightCategory] = s.Score
	}

	type roleCandidate struct {
		title  string
		reason string
		weight int
	}

	roleMappings := []struct {
		categories []string
		role       roleCandidate
	}{
		{
			categories: []string{"リーダーシップ志向", "成長志向", "チャレンジ志向"},
			role: roleCandidate{
				title:  "プロジェクトマネージャー / テックリード",
				reason: "リーダーシップと成長志向を活かし、チームを率いながら技術的課題を解決する役割に向いています",
			},
		},
		{
			categories: []string{"リーダーシップ志向", "技術志向"},
			role: roleCandidate{
				title:  "エンジニアリングマネージャー",
				reason: "技術的な深い理解とリーダーシップを組み合わせ、エンジニアチームを牽引できます",
			},
		},
		{
			categories: []string{"チームワーク志向", "コミュニケーション力"},
			role: roleCandidate{
				title:  "ITコンサルタント / スクラムマスター",
				reason: "コミュニケーション力と協調性を活かし、チーム横断的な課題解決や調整役として活躍できます",
			},
		},
		{
			categories: []string{"技術志向", "細部志向"},
			role: roleCandidate{
				title:  "バックエンド / インフラエンジニア",
				reason: "技術への探求心と細部へのこだわりを活かした、品質重視の技術職に適しています",
			},
		},
		{
			categories: []string{"成長志向", "チャレンジ志向"},
			role: roleCandidate{
				title:  "スタートアップ / 新規事業エンジニア",
				reason: "変化への適応力と挑戦意欲を活かし、スピード感ある環境で大きな裁量を持って働けます",
			},
		},
	}

	type scoredRole struct {
		role  roleCandidate
		total int
	}
	var candidates []scoredRole
	for _, mapping := range roleMappings {
		total := 0
		matched := 0
		for _, cat := range mapping.categories {
			if v, ok := scoreMap[cat]; ok && v > 0 {
				total += v
				matched++
			}
		}
		if matched >= 1 {
			candidates = append(candidates, scoredRole{role: mapping.role, total: total})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].total > candidates[j].total
	})

	maxRoles := 3
	if len(candidates) < maxRoles {
		maxRoles = len(candidates)
	}
	if maxRoles == 0 {
		return "", nil
	}

	var roles []JobSuitabilityRole
	for i := 0; i < maxRoles; i++ {
		roles = append(roles, JobSuitabilityRole{
			Title:  candidates[i].role.title,
			Reason: candidates[i].role.reason,
		})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})
	var strengthParts []string
	for i := 0; i < len(scores) && i < 3; i++ {
		if scores[i].Score > 0 {
			strengthParts = append(strengthParts, strings.TrimSuffix(scores[i].WeightCategory, "志向"))
		}
	}

	strengthText := "複数の強み"
	if len(strengthParts) > 0 {
		strengthText = strings.Join(strengthParts, "・")
	}

	comment := fmt.Sprintf(
		"分析結果から、あなたには%sという強みがあります。これらの特性から、以下の職種が特に向いていると考えられます。",
		strengthText,
	)

	return comment, roles
}

func buildScoreComment(scores AnalysisScores) string {
	var parts []string

	jobPct := scores.JobScore * 100
	switch {
	case jobPct >= 80:
		parts = append(parts, "志望職種への適性が高い")
	case jobPct >= 50:
		parts = append(parts, "志望職種への適性が一定水準ある")
	case jobPct > 0:
		parts = append(parts, "志望職種への理解をさらに深めると良い")
	}

	interestPct := scores.InterestScore * 100
	switch {
	case interestPct >= 80:
		parts = append(parts, "企業への関心・意欲が非常に高い")
	case interestPct >= 50:
		parts = append(parts, "企業への関心・意欲が示されている")
	case interestPct > 0:
		parts = append(parts, "企業への関心をさらに深めると良い")
	}

	aptitudePct := scores.AptitudeScore * 100
	switch {
	case aptitudePct >= 80:
		parts = append(parts, "多面的な適性が高く評価されている")
	case aptitudePct >= 50:
		parts = append(parts, "複数の適性が確認されている")
	case aptitudePct > 0:
		parts = append(parts, "適性をさらに伸ばす余地がある")
	}

	futurePct := scores.FutureScore * 100
	switch {
	case futurePct >= 80:
		parts = append(parts, "将来への展望・成長意欲が強く感じられる")
	case futurePct >= 50:
		parts = append(parts, "将来志向が見られる")
	case futurePct > 0:
		parts = append(parts, "将来ビジョンをより明確にするとよい")
	}

	if len(parts) == 0 {
		return "チャット診断を完了させることで、より詳細な分析コメントが表示されます。"
	}

	comment := strings.Join(parts, "、") + "。"

	finalPct := scores.FinalScore * 100
	switch {
	case finalPct >= 80:
		comment += "総合的に非常に優れたプロフィールです。自信を持って就活に臨んでください。"
	case finalPct >= 60:
		comment += "総合的にバランスの取れたプロフィールです。強みをアピールしながら就活を進めましょう。"
	case finalPct >= 40:
		comment += "いくつかの強みが見られます。診断をさらに深めることでより精度の高いマッチングが可能です。"
	default:
		comment += "診断をさらに進めることで、あなたにぴったりの企業が見つかります。"
	}

	return comment
}
