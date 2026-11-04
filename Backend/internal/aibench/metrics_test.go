package aibench

import (
	"math"
	"testing"
)

func TestStdDev(t *testing.T) {
	tests := []struct {
		name string
		xs   []float64
		want float64
	}{
		{name: "空は0", xs: nil, want: 0},
		{name: "1件は0（-n 1 でも集計が壊れない）", xs: []float64{0.7}, want: 0},
		{name: "同じ値ばかりなら0", xs: []float64{0.5, 0.5, 0.5}, want: 0},
		// 母標準偏差: 平均0.6、偏差 ±0.1 → σ=0.1
		{name: "母標準偏差で計算する", xs: []float64{0.5, 0.7}, want: 0.1},
		// 平均0.6、偏差 -0.2/0/0.2 → σ=sqrt(0.08/3)
		{name: "3件", xs: []float64{0.4, 0.6, 0.8}, want: math.Sqrt(0.08 / 3)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StdDev(tt.xs); math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("StdDev = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPercentile(t *testing.T) {
	xs := []float64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}
	tests := []struct {
		name string
		p    float64
		want float64
	}{
		{name: "p50は5番目（最近傍順位法）", p: 50, want: 500},
		{name: "p95は10番目", p: 95, want: 1000},
		{name: "p0は最小", p: 0, want: 100},
		{name: "p100は最大", p: 100, want: 1000},
		// 補間しないので、必ず実在する値が返る
		{name: "p90は9番目", p: 90, want: 900},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Percentile(xs, tt.p); got != tt.want {
				t.Errorf("Percentile(%v) = %v, want %v", tt.p, got, tt.want)
			}
		})
	}
	if got := Percentile(nil, 50); got != 0 {
		t.Errorf("空スライスは0であるべき: %v", got)
	}
	// 呼び出し元のスライスを並べ替えてしまうと、以降のレイテンシ表示が狂う
	orig := []float64{3, 1, 2}
	_ = Percentile(orig, 50)
	if orig[0] != 3 {
		t.Errorf("入力スライスを破壊している: %v", orig)
	}
}

func TestSpearmanCorrelation(t *testing.T) {
	// ラベル順位（bad=1, mid=2, good=3）とスコアの関係を確かめる
	labels := []float64{1, 1, 2, 2, 3, 3}
	// ラベル側に同順位があるので、スコアが「順序は正しいが全て異なる」場合は
	// 1.0 には届かない（0.9562）。スコア側の同順位がラベル側と一致したときだけ
	// 1.0 になる。どちらの挙動も弁別力の読み方に直結するので両方固定する。
	const distinctScoresMax = 0.9561828874675149
	tests := []struct {
		name   string
		scores []float64
		want   float64
	}{
		{name: "順序は正しいがスコアが全て異なると1.0には届かない", scores: []float64{0.1, 0.2, 0.5, 0.6, 0.8, 0.9}, want: distinctScoresMax},
		{name: "逆順かつスコアが全て異なると-0.956", scores: []float64{0.9, 0.8, 0.6, 0.5, 0.2, 0.1}, want: -distinctScoresMax},
		// 同順位が大量にある系列でも動く（単純な差の2乗の式では計算できない）
		{name: "スコアが全部同じなら0（相関が定義できない）", scores: []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5}, want: 0},
		// スコア側の同順位がラベルの同順位と一致すると、押し下げが起きず1になる
		{name: "ラベルと同じ粒度で並べば1", scores: []float64{0.1, 0.1, 0.5, 0.5, 0.9, 0.9}, want: 1},
		// 良と悪が入れ替わると、中が正しくても -1 まで落ちる
		{name: "良と悪が入れ替わると-1", scores: []float64{0.9, 0.9, 0.5, 0.5, 0.1, 0.1}, want: -1},
		// 1件だけ順序を外すと相関が大きく下がる（劣化の検知に効く感度）
		{name: "良の1件だけ低いと相関が下がる", scores: []float64{0.1, 0.1, 0.5, 0.5, 0.9, 0.05}, want: 0.24618298195866545},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SpearmanCorrelation(labels, tt.scores); math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("SpearmanCorrelation = %v, want %v", got, tt.want)
			}
		})
	}

	// 長さ違い・件数不足は0（例外を投げずに集計を続ける）
	if got := SpearmanCorrelation([]float64{1, 2}, []float64{1}); got != 0 {
		t.Errorf("長さ違いは0であるべき: %v", got)
	}
	if got := SpearmanCorrelation([]float64{1}, []float64{1}); got != 0 {
		t.Errorf("1件は0であるべき: %v", got)
	}
}

func TestApproxTokensJA(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "空は0", in: "", want: 0},
		{name: "日本語は1文字1トークン", in: "私は学生です", want: 6},
		{name: "ASCIIは4文字1トークン", in: "abcdefgh", want: 2},
		{name: "混在", in: "Goを学習", want: 3 + 1}, // 日本語3文字 + "Go"(2文字→1)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ApproxTokensJA(tt.in); got != tt.want {
				t.Errorf("ApproxTokensJA(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestPairwiseAgreement(t *testing.T) {
	// ラベル: 悪=1 中=2 良=3
	tests := []struct {
		name     string
		labels   []float64
		scores   []float64
		want     float64
		wantComp int
	}{
		{name: "完全に順序どおり", labels: []float64{1, 2, 3}, scores: []float64{0.1, 0.5, 0.9}, want: 1, wantComp: 3},
		{name: "完全に逆", labels: []float64{1, 2, 3}, scores: []float64{0.9, 0.5, 0.1}, want: 0, wantComp: 3},
		{name: "全部同点は当てずっぽうと同じ0.5", labels: []float64{1, 2, 3}, scores: []float64{0.5, 0.5, 0.5}, want: 0.5, wantComp: 3},
		{name: "同じラベルの組は比べない", labels: []float64{3, 3, 1}, scores: []float64{0.2, 0.9, 0.1}, want: 1, wantComp: 2},
		{name: "1組だけ逆転(良の一方が悪より低い)", labels: []float64{3, 3, 1}, scores: []float64{0.9, 0.05, 0.1}, want: 0.5, wantComp: 2},
		{name: "ラベルが1種類なら比べる組が無い", labels: []float64{2, 2}, scores: []float64{0.1, 0.9}, want: 0, wantComp: 0},
		{name: "長さが違う入力は0を返す", labels: []float64{1, 2}, scores: []float64{0.1}, want: 0, wantComp: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, comp := PairwiseAgreement(tt.labels, tt.scores)
			if math.Abs(got-tt.want) > 1e-9 || comp != tt.wantComp {
				t.Errorf("PairwiseAgreement = %v (%d組), want %v (%d組)", got, comp, tt.want, tt.wantComp)
			}
		})
	}
}
