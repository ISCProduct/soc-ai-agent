package matching_test

// マッチ度計算のテスト。
//
// 以前はこのファイルが式をコピーして持っており、しかも本体がシグモイドへ
// 変わった後もコピー側は 100-|diff| の線形式のままだった。
// 「テストは通るが本体は別物」という状態だったので、
// CalculateCategoryMatch の公開(#1027)に合わせて本体呼び出しへ差し替える。
//
// 具体値ではなく満たすべき性質で検証する。式の係数調整で落ちない一方、
// 単調性や対称性が壊れれば検出できる。

import (
	"math"
	"testing"

	"Backend/internal/services/matching"

	"github.com/stretchr/testify/assert"
)

// 完全一致が最大値になること。
func TestCalculateCategoryMatch_PerfectMatchIsHighest(t *testing.T) {
	perfect := matching.CalculateCategoryMatch(80, 80)
	for _, diff := range []float64{1, 5, 20, 50, 100} {
		got := matching.CalculateCategoryMatch(80, math.Max(0, 80-diff))
		assert.Less(t, got, perfect,
			"差が %v あるのに完全一致以上のスコアになっている", diff)
	}
}

// 差が大きいほどスコアが下がること（単調減少）。
func TestCalculateCategoryMatch_MonotonicallyDecreasing(t *testing.T) {
	prev := matching.CalculateCategoryMatch(100, 100)
	for w := 99.0; w >= 0; w -= 1 {
		got := matching.CalculateCategoryMatch(100, w)
		assert.LessOrEqual(t, got, prev,
			"重視度 %v でスコアが上がった（単調性が崩れている）", w)
		prev = got
	}
}

// 0〜100 に収まること。
func TestCalculateCategoryMatch_Bounded(t *testing.T) {
	for _, tt := range []struct{ user, weight float64 }{
		{0, 100}, {100, 0}, {0, 0}, {100, 100}, {50, 50}, {-10, 200},
	} {
		got := matching.CalculateCategoryMatch(tt.user, tt.weight)
		assert.GreaterOrEqual(t, got, 0.0, "user=%v weight=%v", tt.user, tt.weight)
		assert.LessOrEqual(t, got, 100.0, "user=%v weight=%v", tt.user, tt.weight)
	}
}

// 差の符号によらず同じスコアになること（対称性）。
func TestCalculateCategoryMatch_SymmetricDifference(t *testing.T) {
	for _, diff := range []float64{5, 20, 40} {
		above := matching.CalculateCategoryMatch(50+diff, 50)
		below := matching.CalculateCategoryMatch(50-diff, 50)
		assert.InDelta(t, above, below, 0.0001,
			"差 %v の上下でスコアが違う（対称でない）", diff)
	}
}

// 完全不一致が最小値になること。
func TestCalculateCategoryMatch_WorstCase(t *testing.T) {
	worst := matching.CalculateCategoryMatch(0, 100)
	assert.Less(t, worst, matching.CalculateCategoryMatch(0, 99))
	assert.GreaterOrEqual(t, worst, 0.0)
}
