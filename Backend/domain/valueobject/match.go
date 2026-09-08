package valueobject

import (
	"fmt"
	"strings"
)

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

// カテゴリ名の表記揺れを正典へ写像する（#929）。
//
// 正典は上の10種類だが、シードデータ・質問体系・面接スコアの写像が
// それぞれ独自の表記を持っており、user_weight_scores に正典外の値が
// 書き込まれていた。マッチング側は scoreMap を正典キーで引くため、
// 不一致の行は引かれず、代わりに中立50が使われる
// （matching_service.go の scoredMatch）。エラーもログも出ないまま
// ユーザーの実スコアが捨てられる。
//
// 表記を1箇所に集約し、保存経路で必ず通すことで再発を止める。
var weightCategoryAliases = map[string]WeightCategory{
	// 「志向」サフィックスの欠落
	"チームワーク":    CategoryTeamwork,
	"リーダーシップ":   CategoryLeadership,
	"創造性":       CategoryCreativity,
	"チャレンジ":     CategoryChallenge,
	"技術":        CategoryTechnical,
	"安定":        CategoryStability,
	"成長":        CategoryGrowth,
	"細部":        CategoryDetail,
	"コミュニケーション": CategoryCommunication,

	// 「力」「能力」の揺れ
	"コミュニケーション能力": CategoryCommunication,

	// チャットの質問体系(chat_question_fallback.go)が持っていた別分類。
	// 質問文の切り口としての名前であり、評価軸としては正典へ寄せる。
	//
	// ここから下は「表記揺れの吸収」ではなく「意味的な統合」である点に注意。
	// 正典10軸に対応する軸が無いものを最も近い軸へ寄せているため、
	// スコアの意味が元の質問の意図とは変わる。
	//
	// 特に ビジネス思考・目標志向 -> 成長志向 は消去法で、対応が弱い。
	// 正典に事業志向の軸が無いためどこへ寄せても不正解になる。
	// 詳細と切り離す条件は docs/wiki/scoring.md「カテゴリの正典と別名」を参照。
	"創造性・発想力":     CategoryCreativity,
	"学習意欲・成長志向":   CategoryGrowth,
	"問題解決力":       CategoryTechnical,
	"分析思考":        CategoryTechnical,
	"計画性・実行力":     CategoryDetail,
	"ストレス耐性・粘り強さ": CategoryChallenge,
	"ビジネス思考・目標志向": CategoryGrowth,
}

// NormalizeWeightCategory は表記揺れを正典へ寄せる。
// 正典にも別名表にも無い場合は ok=false を返す。呼び出し側で弾くこと。
func NormalizeWeightCategory(raw string) (WeightCategory, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	c := WeightCategory(s)
	for _, known := range AllWeightCategories() {
		if c == known {
			return known, true
		}
	}
	if mapped, ok := weightCategoryAliases[s]; ok {
		return mapped, true
	}
	return "", false
}

// ParseWeightCategory は NormalizeWeightCategory のエラー版。
// 保存経路の入口で使い、正典外の値がDBへ入るのを防ぐ。
func ParseWeightCategory(raw string) (WeightCategory, error) {
	c, ok := NormalizeWeightCategory(raw)
	if !ok {
		return "", fmt.Errorf("未知の重みカテゴリです: %q", raw)
	}
	return c, nil
}
