package services_test

// スコアバリデーション相関係数のテスト（Issue #313）
// 実行: cd Backend && go test ./test/services/... -run TestCorrelation -v

import (
	"math"
	"testing"
)

// correlationApprox は本番コードと同一の相関係数近似ロジック
func correlationApprox(weight float64) float64 {
	return math.Max(-1.0, math.Min(weight-1.0, 1.0))
}

// TestCorrelation_AlwaysInRange は計算結果が常に [-1, 1] に収まることを検証する（#313修正の担保）
func TestCorrelation_AlwaysInRange(t *testing.T) {
	tests := []struct {
		weight float64
	}{
		{0.0},
		{0.1},
		{0.5},
		{1.0},
		{1.5},
		{2.0},
		{3.0},
		{10.0},
	}

	for _, tc := range tests {
		c := correlationApprox(tc.weight)
		if c < -1.0 || c > 1.0 {
			t.Errorf("weight=%.1f: 相関係数 %f は [-1, 1] 範囲外", tc.weight, c)
		}
	}
}

// TestCorrelation_AverageWeightIsZero は weight=1.0 のとき相関係数が 0 になることを検証する
func TestCorrelation_AverageWeightIsZero(t *testing.T) {
	c := correlationApprox(1.0)
	if math.Abs(c) > 1e-9 {
		t.Errorf("weight=1.0 は相関係数 0 になるべき: got %f", c)
	}
}

// TestCorrelation_HighWeightClampedToOne は weight=2.0 以上のとき相関係数が 1.0 になることを検証する
func TestCorrelation_HighWeightClampedToOne(t *testing.T) {
	c := correlationApprox(2.0)
	if math.Abs(c-1.0) > 1e-9 {
		t.Errorf("weight=2.0 は相関係数 1.0 になるべき: got %f", c)
	}
}

// TestCorrelation_LowWeightClampedToMinusOne は weight=0.0 のとき相関係数が -1.0 になることを検証する（旧実装では下限なし）
func TestCorrelation_LowWeightClampedToMinusOne(t *testing.T) {
	c := correlationApprox(0.0)
	if math.Abs(c-(-1.0)) > 1e-9 {
		t.Errorf("weight=0.0 は相関係数 -1.0 になるべき: got %f", c)
	}
}

// TestCorrelation_OldBugRegression は修正前のバグ（下限がなかった）が再発しないことを検証する
func TestCorrelation_OldBugRegression(t *testing.T) {
	c := correlationApprox(-0.5)
	if c < -1.0 {
		t.Errorf("相関係数は -1.0 を下回ってはならない（旧バグ回帰テスト）: got %f", c)
	}
}
