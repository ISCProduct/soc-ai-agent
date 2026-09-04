package valueobject

import "fmt"

// WeightCategory 適性診断の重みカテゴリを表す値オブジェクト
type WeightCategory string

const (
	CategoryTechnical     WeightCategory = "技術志向"
	CategoryTeamwork      WeightCategory = "チームワーク志向"
	CategoryLeadership    WeightCategory = "リーダーシップ志向"
	CategoryCreativity    WeightCategory = "創造性志向"
	CategoryStability     WeightCategory = "安定志向"
	CategoryGrowth        WeightCategory = "成長志向"
	CategoryWorkLife      WeightCategory = "ワークライフバランス"
	CategoryChallenge     WeightCategory = "チャレンジ志向"
	CategoryDetail        WeightCategory = "細部志向"
	CategoryCommunication WeightCategory = "コミュニケーション力"
)

// AllWeightCategories 全カテゴリのリストを返す
func AllWeightCategories() []WeightCategory {
	return []WeightCategory{
		CategoryTechnical,
		CategoryTeamwork,
		CategoryLeadership,
		CategoryCreativity,
		CategoryStability,
		CategoryGrowth,
		CategoryWorkLife,
		CategoryChallenge,
		CategoryDetail,
		CategoryCommunication,
	}
}

// MatchScore ユーザーと企業のカテゴリ別マッチ度を表す値オブジェクト
type MatchScore struct {
	Category      WeightCategory
	UserScore     float64 // ユーザースコア (0-100)
	CompanyWeight float64 // 企業重視度 (0-100)
	MatchDegree   float64 // マッチ度 (0-100)
}

// NewMatchScore カテゴリマッチ度を計算して生成する
// マッチ度 = 100 - |ユーザースコア - 企業重視度|
func NewMatchScore(category WeightCategory, userScore, companyWeight float64) MatchScore {
	diff := userScore - companyWeight
	if diff < 0 {
		diff = -diff
	}
	matchDegree := 100.0 - diff
	if matchDegree < 0 {
		matchDegree = 0
	}
	return MatchScore{
		Category:      category,
		UserScore:     userScore,
		CompanyWeight: companyWeight,
		MatchDegree:   matchDegree,
	}
}

// IsHighMatch マッチ度が高い（80以上）かどうか
func (m MatchScore) IsHighMatch() bool {
	return m.MatchDegree >= 80
}

// String カテゴリとマッチ度の文字列表現
func (m MatchScore) String() string {
	return fmt.Sprintf("%s: %.1f%%", m.Category, m.MatchDegree)
}
