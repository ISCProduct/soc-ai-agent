package aibench

import (
	"math"
	"slices"
	"sort"
)

// Mean は平均。空なら0。
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// StdDev は母標準偏差（n で割る）。
//
// 標本標準偏差（n-1）ではなく母標準偏差にするのは、ここで測るのが
// 「実際に行った n 回のばらつき」そのものであって母集団の推定ではないため。
// n=1 のとき 0 を返し、-n 1 で実行しても集計が壊れない。
func StdDev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := Mean(xs)
	var sum float64
	for _, x := range xs {
		d := x - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(xs)))
}

// Percentile は最近傍順位法でパーセンタイルを返す（p は 0〜100）。
//
// 補間しないのは、レイテンシの p95 を「実際に観測した値のどれか」に
// 保ちたいため。件数が少ないとき補間値は実在しない待ち時間になる。
func Percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := slices.Clone(xs)
	sort.Float64s(sorted)
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	// 順位 = ceil(p/100 * n)、1始まり
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

// SpearmanCorrelation は順位相関（同順位は平均順位で処理する）。
//
// 弁別力の指標にこれを使うのは、モデルごとにスコアの中心がずれるため。
// 「良は7点以上」のような絶対値の一致を求めると、中心が1点下がっただけの
// モデルを不合格にしてしまう。見たいのは「良・中・悪の順に並ぶか」だけ。
//
// ゴールドラベルは3値しか無く同順位が大量に出るので、単純な差の2乗の式
// （6Σd²/n(n²-1)）は使えない。順位に変換した上でピアソン相関を取る。
// 返り値は -1〜1。片方の系列が定数のとき（全部同じラベル/全部同じスコア）は
// 相関が定義できないため 0 を返す。
func SpearmanCorrelation(xs, ys []float64) float64 {
	if len(xs) != len(ys) || len(xs) < 2 {
		return 0
	}
	return pearson(averageRanks(xs), averageRanks(ys))
}

// averageRanks は値を順位へ変換する。同じ値には平均順位を与える。
func averageRanks(xs []float64) []float64 {
	type pair struct {
		v float64
		i int
	}
	ps := make([]pair, len(xs))
	for i, x := range xs {
		ps[i] = pair{x, i}
	}
	sort.Slice(ps, func(a, b int) bool { return ps[a].v < ps[b].v })

	ranks := make([]float64, len(xs))
	for i := 0; i < len(ps); {
		j := i
		for j+1 < len(ps) && ps[j+1].v == ps[i].v {
			j++
		}
		// i..j が同順位。1始まりの順位の平均を配る
		avg := (float64(i+1) + float64(j+1)) / 2
		for k := i; k <= j; k++ {
			ranks[ps[k].i] = avg
		}
		i = j + 1
	}
	return ranks
}

func pearson(xs, ys []float64) float64 {
	mx, my := Mean(xs), Mean(ys)
	var num, dx, dy float64
	for i := range xs {
		a := xs[i] - mx
		b := ys[i] - my
		num += a * b
		dx += a * a
		dy += b * b
	}
	if dx == 0 || dy == 0 {
		return 0
	}
	return num / math.Sqrt(dx*dy)
}

// ApproxTokensJA は日本語混在テキストのトークン数を概算する。
//
// 実トークン数が API から返らない経路（ES添削は RAG 経由で usage が
// 手に入らない）のコスト概算にだけ使う。o200k_base では日本語はおよそ
// 1文字1トークン、ASCII は4文字1トークンなので、その比で数える。
// 概算なので、コストの絶対値ではなく実行間の比較に使うこと。
func ApproxTokensJA(s string) int {
	ascii, other := 0, 0
	for _, r := range s {
		if r < 128 {
			ascii++
		} else {
			other++
		}
	}
	return other + (ascii+3)/4
}
