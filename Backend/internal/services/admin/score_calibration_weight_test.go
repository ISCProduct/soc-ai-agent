package admin

import (
	"math"
	"testing"
)

func TestWeightFromPassFailSignal_HighScorePredictsPass(t *testing.T) {
	// 通過側が +40 点高い → 重み > 1
	w := WeightFromPassFailSignal(40, 100)
	if w <= 1.0 {
		t.Fatalf("予測力のある軸は weight>1 のはず: got %.3f", w)
	}
	if w > calibrationWeightMax {
		t.Fatalf("clamp 超え: %.3f", w)
	}
}

func TestWeightFromPassFailSignal_HighScorePredictsFail(t *testing.T) {
	w := WeightFromPassFailSignal(-40, 100)
	if w >= 1.0 {
		t.Fatalf("逆信号の軸は weight<1 のはず: got %.3f", w)
	}
	if w < calibrationWeightMin {
		t.Fatalf("clamp 下回り: %.3f", w)
	}
}

func TestWeightFromPassFailSignal_ShrinksTowardOneWhenSparse(t *testing.T) {
	strong := WeightFromPassFailSignal(40, 100)
	weak := WeightFromPassFailSignal(40, 10)
	if math.Abs(weak-1.0) >= math.Abs(strong-1.0) {
		t.Fatalf("少サンプルの方が 1.0 に近いはず: strong=%.3f weak=%.3f", strong, weak)
	}
}

func TestStepToward_CapsDelta(t *testing.T) {
	got := stepToward(1.0, 2.0, 0.3)
	if math.Abs(got-1.3) > 1e-9 {
		t.Fatalf("stepToward = %.3f, want 1.3", got)
	}
	got = stepToward(1.0, 0.5, 0.3)
	if math.Abs(got-0.7) > 1e-9 {
		t.Fatalf("stepToward down = %.3f, want 0.7", got)
	}
}
