// Package teacher は教員（担当校を持つ管理者）向けの生徒分析を提供する（#1027）。
package teacher

import (
	"sort"

	"Backend/domain/valueobject"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/matching"
)

// TopCategory は生徒のスコアが高いカテゴリ1件。
type TopCategory struct {
	Category string  `json:"category"`
	Score    float64 `json:"score"`
}

// SuitedIndustry は生徒に向いている業界1件。
type SuitedIndustry struct {
	IndustryID   uint    `json:"industry_id"`
	IndustryName string  `json:"industry_name"`
	Score        float64 `json:"score"`
}

// StudentTendency は生徒1人ぶんの分析結果。
type StudentTendency struct {
	UserID           uint             `json:"user_id"`
	Name             string           `json:"name"`
	Email            string           `json:"email"`
	TypeLabel        string           `json:"type_label"`
	TopCategories    []TopCategory    `json:"top_categories"`
	SuitedIndustries []SuitedIndustry `json:"suited_industries"`
	// 低マッチのまま進行中の応募（#1028）。無ければ空。
	LowMatchApplications []repositories.LowMatchApplication `json:"low_match_applications,omitempty"`
	// DataAvailable が false のときタイプも業界も参考にならない。
	// UI 側で「分析データ不足」と出す（PRD 境界値）。
	DataAvailable bool `json:"data_available"`
}

const (
	topCategoryCount    = 3 // タイプ判定と表示に使う上位カテゴリ数
	suitedIndustryCount = 3 // 返す業界数（PRD: TOP3）
)

// typeLabels は最上位カテゴリからタイプ名への対応。
//
// 断定的に見せない文言配慮は UI 側でも行うが（PRD リスク欄）、
// ここでも人格の断定を避け「傾向」を表す語に留める。
var typeLabels = map[string]string{
	"技術志向":       "技術探究タイプ",
	"チームワーク志向":   "協働タイプ",
	"リーダーシップ志向":  "牽引タイプ",
	"創造性志向":      "発想タイプ",
	"安定志向":       "堅実タイプ",
	"成長志向":       "成長意欲タイプ",
	"ワークライフバランス": "バランス重視タイプ",
	"チャレンジ志向":    "挑戦タイプ",
	"細部志向":       "緻密タイプ",
	"コミュニケーション力": "対話タイプ",
}

// TypeLabelForCategory は正典カテゴリからタイプ名を返す。
func TypeLabelForCategory(category string) string {
	if label, ok := typeLabels[category]; ok {
		return label
	}
	return "傾向データ不足"
}

// RankCategories はスコアの高い順に上位カテゴリを返す（#1027）。
//
// 同点時は正典(AllWeightCategories)の並び順で決める。
// map の反復順に任せると同じ生徒でも表示順が変わるため。
func RankCategories(scores map[string]float64, limit int) []TopCategory {
	if len(scores) == 0 || limit <= 0 {
		return nil
	}
	order := map[string]int{}
	for i, c := range valueobject.AllWeightCategories() {
		order[string(c)] = i
	}

	ranked := make([]TopCategory, 0, len(scores))
	for c, s := range scores {
		ranked = append(ranked, TopCategory{Category: c, Score: s})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		oi, oki := order[ranked[i].Category]
		oj, okj := order[ranked[j].Category]
		switch {
		case oki && okj:
			return oi < oj
		case oki:
			return true
		case okj:
			return false
		default:
			return ranked[i].Category < ranked[j].Category
		}
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

// RankIndustries は生徒のスコアと業界プロファイルからマッチ度の高い業界を返す。
//
// マッチ度は企業マッチングと同じ matching.CalculateCategoryMatch を使う。
// 別式で再実装すると企業側の結果と食い違う（PRD 非機能要件）。
// 未設定の業界は中立プロファイルで評価し、エラーにしない。
func RankIndustries(
	scores map[string]float64,
	industries []repositories.IndustryOption,
	profiles map[uint]*models.IndustryWeightProfile,
	limit int,
) []SuitedIndustry {
	if limit <= 0 || len(industries) == 0 {
		return nil
	}
	categories := valueobject.AllWeightCategories()

	ranked := make([]SuitedIndustry, 0, len(industries))
	for _, ind := range industries {
		profile := profiles[ind.ID]
		if profile == nil {
			profile = models.NeutralIndustryWeightProfile(ind.ID)
		}
		weights := profile.WeightByCategory()

		total := 0.0
		for _, c := range categories {
			category := string(c)
			// 未評価カテゴリは中立50。企業マッチング側(scoredMatch)と同じ扱い。
			userScore, ok := scores[category]
			if !ok {
				userScore = 50
			}
			total += matching.CalculateCategoryMatch(userScore, weights[category])
		}
		ranked = append(ranked, SuitedIndustry{
			IndustryID:   ind.ID,
			IndustryName: ind.Name,
			Score:        total / float64(len(categories)),
		})
	}

	// 同点は業界IDの昇順で固定する。表示順がリクエストごとに変わらないように。
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].IndustryID < ranked[j].IndustryID
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

// minMeasuredCategories は分析結果を出すのに必要な計測カテゴリ数。
//
// 10カテゴリ中1つしか計測されていない生徒に断定的なタイプ名を出すと、
// 残り9カテゴリを中立50で埋めた業界TOP3と合わせて表示のほとんどがノイズになる。
// PRD が避けたい「断定」そのものなので、下回るときはデータ不足として扱う。
const minMeasuredCategories = 3

// hasMeaningfulScores は分析に足る実測があるかを判定する（#1027）。
//
// スコアが1件も無い場合に加え、全カテゴリが0点の場合もデータ不足とする。
// 実データには「7カテゴリすべて score=0」の生徒が存在し、
// そのままだと確信ありげなタイプ名が教員に表示される。
func hasMeaningfulScores(scores map[string]float64) bool {
	if len(scores) < minMeasuredCategories {
		return false
	}
	for _, v := range scores {
		if v > 0 {
			return true
		}
	}
	return false
}

// BuildTendency は1人ぶんの分析結果を組み立てる。
// スコアが1件も無い生徒は DataAvailable=false にして、
// タイプも業界も出さない（誤った断定を避ける）。
func BuildTendency(
	userID uint,
	name, email string,
	scores map[string]float64,
	industries []repositories.IndustryOption,
	profiles map[uint]*models.IndustryWeightProfile,
) StudentTendency {
	t := StudentTendency{UserID: userID, Name: name, Email: email}
	if !hasMeaningfulScores(scores) {
		t.TypeLabel = "分析データ不足"
		return t
	}
	t.DataAvailable = true
	t.TopCategories = RankCategories(scores, topCategoryCount)
	if len(t.TopCategories) > 0 {
		t.TypeLabel = TypeLabelForCategory(t.TopCategories[0].Category)
	}
	t.SuitedIndustries = RankIndustries(scores, industries, profiles, suitedIndustryCount)
	return t
}
