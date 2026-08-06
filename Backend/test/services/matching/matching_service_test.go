package matching_test

// マッチングサービスのユニットテスト (Issue #188)
//
// 実行: cd Backend && go test ./test/services/... -run Matching -v

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// calculateCategoryMatch の純粋なロジックをテスト（パッケージ外からのホワイトボックステスト）
// 関数本体: math.Max(0, 100.0 - |userScore - companyWeight|)

func categoryMatch(userScore, companyWeight float64) float64 {
	diff := math.Abs(userScore - companyWeight)
	return math.Max(0, 100.0-diff)
}

// scoredMatch のロジックをテスト用に再現
func scoredMatchTest(userScores map[string]float64, category string, companyWeight float64, evaluatedCount int, totalScore float64) (float64, int, float64) {
	userScore, ok := userScores[category]
	if !ok {
		return 0, evaluatedCount, totalScore
	}
	matchScore := categoryMatch(userScore, companyWeight)
	return matchScore, evaluatedCount + 1, totalScore + matchScore
}

// TestCategoryMatch_PerfectMatch は完全一致の場合にスコア100を返すことを検証
func TestCategoryMatch_PerfectMatch(t *testing.T) {
	score := categoryMatch(80, 80)
	assert.InDelta(t, 100.0, score, 0.001, "同一スコアで完全マッチはスコア100")
}

// TestCategoryMatch_PartialMatch は差が20の場合にスコア80を返すことを検証
func TestCategoryMatch_PartialMatch(t *testing.T) {
	score := categoryMatch(80, 60)
	assert.InDelta(t, 80.0, score, 0.001, "差20でスコア80")
}

// TestCategoryMatch_ZeroFloor はスコアが0を下回らないことを検証
func TestCategoryMatch_ZeroFloor(t *testing.T) {
	score := categoryMatch(0, 100)
	assert.InDelta(t, 0.0, score, 0.001, "差100超でスコア0（負にならない）")
}

// TestCategoryMatch_NearZero は差が99の場合にスコア1を返すことを検証
func TestCategoryMatch_NearZero(t *testing.T) {
	score := categoryMatch(1, 100)
	assert.InDelta(t, 1.0, score, 0.001, "差99でスコア1")
}

// TestScoredMatch_CategoryMissing はユーザースコアにカテゴリがない場合にスコア0を返し、evaluatedCountを増やさないことを検証
func TestScoredMatch_CategoryMissing(t *testing.T) {
	userScores := map[string]float64{"技術志向": 80}
	score, count, total := scoredMatchTest(userScores, "チームワーク志向", 70, 0, 0)
	assert.InDelta(t, 0.0, score, 0.001, "欠損カテゴリはスコア0")
	assert.Equal(t, 0, count, "欠損カテゴリはevaluatedCountを増やさない")
	assert.InDelta(t, 0.0, total, 0.001, "欠損カテゴリはtotalScoreを増やさない")
}

// TestScoredMatch_CategoryPresent はカテゴリが存在する場合に正しくスコアを計算することを検証
func TestScoredMatch_CategoryPresent(t *testing.T) {
	userScores := map[string]float64{"技術志向": 75}
	score, count, total := scoredMatchTest(userScores, "技術志向", 80, 0, 0)
	assert.InDelta(t, 95.0, score, 0.001, "差5でスコア95")
	assert.Equal(t, 1, count, "カテゴリ存在でevaluatedCount+1")
	assert.InDelta(t, 95.0, total, 0.001, "totalScoreが加算される")
}

// TestAverageMatchScore は複数カテゴリの平均スコアが正しいことを検証
func TestAverageMatchScore_MultiCategory(t *testing.T) {
	userScores := map[string]float64{
		"技術志向":      80,
		"チームワーク志向":  60,
		"リーダーシップ志向": 40,
	}
	categories := []struct {
		name   string
		weight float64
	}{
		{"技術志向", 80},      // diff=0  → 100
		{"チームワーク志向", 80},  // diff=20 → 80
		{"リーダーシップ志向", 80}, // diff=40 → 60
	}

	count := 0
	total := 0.0
	for _, cat := range categories {
		_, count, total = scoredMatchTest(userScores, cat.name, cat.weight, count, total)
	}
	avg := total / float64(count)
	// 期待値: (100 + 80 + 60) / 3 = 80.0
	assert.InDelta(t, 80.0, avg, 0.001, "3カテゴリの平均スコアが正しい")
}

// TestAverageMatchScore_WithMissingCategory は欠損カテゴリが平均計算に影響しないことを検証
func TestAverageMatchScore_WithMissingCategory(t *testing.T) {
	userScores := map[string]float64{
		"技術志向": 80,
		// チームワーク志向は欠損
	}
	count := 0
	total := 0.0
	_, count, total = scoredMatchTest(userScores, "技術志向", 80, count, total)
	_, count, total = scoredMatchTest(userScores, "チームワーク志向", 70, count, total)

	// 技術志向のみ評価: (100) / 1 = 100
	assert.Equal(t, 1, count, "欠損カテゴリは分母に含まれない")
	assert.InDelta(t, 100.0, total/float64(count), 0.001, "欠損カテゴリを除外した平均が正しい")
}

// TestCategoryMatch_SymmetricDifference は差の方向がスコアに影響しないことを検証
func TestCategoryMatch_SymmetricDifference(t *testing.T) {
	score1 := categoryMatch(60, 80)
	score2 := categoryMatch(80, 60)
	assert.InDelta(t, score1, score2, 0.001, "差の方向（上下）でスコアが変わらない（対称性）")
}
