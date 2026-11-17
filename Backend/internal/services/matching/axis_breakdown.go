package matching

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/shared"
)

// 軸別のマッチの内訳（SOCAIAGENT-611 / #2026）。
//
// 推薦の一覧は「噛み合った軸」を1行で出すだけで、自分のどの傾向がどの企業要件に効いたかが分からなかった。
// 推薦1件について、10軸ごとに「あなたのスコア・企業の重視度・一致度」を返し、強みと補強ポイントを
// 決まった規則で短く言葉にする（AI では書かない。誇張しない・毎回同じ説明にするため）。
// 推薦順位を変えるものではなく、自己理解のための説明。

// AxisDef は10軸の定義（API のキー・表示名・企業の重視度の取り出し方）。
type AxisDef struct {
	Key      string
	Category string // user_weight_scores.weight_category と同じ名前
	weight   func(p *models.CompanyWeightProfile) int
}

// Axes は10軸。並びは推薦の category_scores と同じ。
var Axes = []AxisDef{
	{"technical", "技術志向", func(p *models.CompanyWeightProfile) int { return p.TechnicalOrientation }},
	{"teamwork", "チームワーク志向", func(p *models.CompanyWeightProfile) int { return p.TeamworkOrientation }},
	{"leadership", "リーダーシップ志向", func(p *models.CompanyWeightProfile) int { return p.LeadershipOrientation }},
	{"creativity", "創造性志向", func(p *models.CompanyWeightProfile) int { return p.CreativityOrientation }},
	{"stability", "安定志向", func(p *models.CompanyWeightProfile) int { return p.StabilityOrientation }},
	{"growth", "成長志向", func(p *models.CompanyWeightProfile) int { return p.GrowthOrientation }},
	{"work_life", "ワークライフバランス", func(p *models.CompanyWeightProfile) int { return p.WorkLifeBalance }},
	{"challenge", "チャレンジ志向", func(p *models.CompanyWeightProfile) int { return p.ChallengeSeeking }},
	{"detail", "細部志向", func(p *models.CompanyWeightProfile) int { return p.DetailOrientation }},
	{"communication", "コミュニケーション力", func(p *models.CompanyWeightProfile) int { return p.CommunicationSkill }},
}

// 言葉にするときの目安。
const (
	// axisHigh 以上を「高め」「重視している」とみなす
	axisHigh = 60
	// strengthMaxGap 以内の差なら「近い」とみなす
	strengthMaxGap = 15
	// gapMin 以上、企業の重視度を下回っていれば補強ポイントにする
	gapMin = 20
	// maxNotes は強み・補強ポイントそれぞれの最大件数（並べすぎない）
	maxNotes = 3
)

// AxisRow は1軸の内訳。UserScore・Match が nil の軸は、まだ診断で測れていない（総合の一致度にも使っていない）。
type AxisRow struct {
	Key           string   `json:"key"`
	Label         string   `json:"label"`
	UserScore     *float64 `json:"user_score"`
	CompanyWeight int      `json:"company_weight"`
	Match         *float64 `json:"match"`
}

// AxisNote は強み・補強ポイントの1件。
type AxisNote struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

// AxisBreakdown は推薦1件の軸別の内訳。
type AxisBreakdown struct {
	MatchID     uint       `json:"match_id"`
	CompanyID   uint       `json:"company_id"`
	CompanyName string     `json:"company_name"`
	Axes        []AxisRow  `json:"axes"`
	Strengths   []AxisNote `json:"strengths"`
	Gaps        []AxisNote `json:"gaps"`
	// Unmeasured は、企業が重視しているのに、まだ診断で測れていない軸の表示名。
	Unmeasured []string `json:"unmeasured"`
}

// GetAxisBreakdown は本人の推薦1件（matchID）について、軸別の内訳を返す。
// 他人の推薦・存在しない推薦・企業のプロファイルが無い推薦は shared.ErrNotFound（他人の ID の有無を探らせない）。
//
// スコアは推薦の計算（CalculateMatching）と同じく、その診断のスコアに適性検査を合流させたものを使い、
// 一致度も同じ式（CalculateCategoryMatch）で今のスコアから計算し直す。
func (s *MatchingService) GetAxisBreakdown(matchID, userID uint) (*AxisBreakdown, error) {
	match, err := s.matchRepo.FindByID(matchID)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && match.UserID != userID) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	profiles, err := s.companyRepo.GetWeightProfilesByCompanyIDs([]uint{match.CompanyID})
	if err != nil {
		return nil, err
	}
	profile := profiles[match.CompanyID]
	if profile == nil {
		return nil, shared.ErrNotFound
	}
	scores, err := s.userWeightScoreRepo.FindByUserAndSession(userID, match.SessionID)
	if err != nil {
		return nil, err
	}
	scoreMap := make(map[string]float64, len(scores))
	for _, sc := range scores {
		scoreMap[sc.WeightCategory] = float64(sc.Score)
	}
	scoreMap = s.mergeAssessmentScores(userID, scoreMap)

	out := BuildAxisBreakdown(scoreMap, profile)
	out.MatchID = match.ID
	out.CompanyID = match.CompanyID
	out.CompanyName = match.Company.Name
	return out, nil
}

// BuildAxisBreakdown はスコアと企業の重視度から、軸別の内訳と言葉を組み立てる。
func BuildAxisBreakdown(userScores map[string]float64, profile *models.CompanyWeightProfile) *AxisBreakdown {
	out := &AxisBreakdown{Axes: make([]AxisRow, 0, len(Axes)), Strengths: []AxisNote{}, Gaps: []AxisNote{}, Unmeasured: []string{}}
	type scored struct {
		note AxisNote
		rank float64
	}
	var strengths, gaps []scored
	for _, a := range Axes {
		weight := a.weight(profile)
		row := AxisRow{Key: a.Key, Label: a.Category, CompanyWeight: weight}
		user, ok := userScores[a.Category]
		if !ok {
			out.Axes = append(out.Axes, row)
			if weight >= axisHigh {
				out.Unmeasured = append(out.Unmeasured, a.Category)
			}
			continue
		}
		user = math.Round(user)
		match := CalculateCategoryMatch(user, float64(weight))
		row.UserScore, row.Match = &user, &match
		out.Axes = append(out.Axes, row)

		gap := float64(weight) - user
		switch {
		case user >= axisHigh && weight >= axisHigh && math.Abs(gap) <= strengthMaxGap:
			strengths = append(strengths, scored{AxisNote{a.Key, fmt.Sprintf(
				"あなたの「%s」（%d）は、この企業が重視している度合い（%d）と近く、一致しています。",
				a.Category, int(user), weight)}, match})
		case weight >= axisHigh && gap >= gapMin:
			gaps = append(gaps, scored{AxisNote{a.Key, fmt.Sprintf(
				"この企業は「%s」を重視しています（%d）。あなたの今のスコアは %d です。関係する経験を具体的に話せるようにすると、伝わりやすくなります。",
				a.Category, weight, int(user))}, gap})
		}
	}
	// 強みは一致度の高い順、補強ポイントは差の大きい順。同じなら軸の並び順（安定ソート）
	sort.SliceStable(strengths, func(i, j int) bool { return strengths[i].rank > strengths[j].rank })
	sort.SliceStable(gaps, func(i, j int) bool { return gaps[i].rank > gaps[j].rank })
	for i := range min(len(strengths), maxNotes) {
		out.Strengths = append(out.Strengths, strengths[i].note)
	}
	for i := range min(len(gaps), maxNotes) {
		out.Gaps = append(out.Gaps, gaps[i].note)
	}
	return out
}
