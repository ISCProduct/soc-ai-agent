package company

import (
	"math"

	"Backend/internal/models"
)

// 古いプロファイルの作り直し（SOCAIAGENT-670）。
//
// 企業プロファイルの生成は #1378（2026-09-20）で「平均的な日本企業と比べた相対評価」になり、
// 軸が 100 に張り付かなくなった。それ以前に作られたプロファイルは、多くの軸が 100 に張り付いたままで、
// 軸ごとの企業間の幅が無く、企業を区別する材料にならない。形（100 の軸の数）で、作り直す候補を決める。
const (
	// LegacySaturatedAxes は、100 点の軸がこの数以上のプロファイルを、作り直しの候補にする。
	// 相対評価のプロンプトは、80 以上を最大 3 項目に制限するので、100 点が 4 軸以上ある形は、旧い生成の特徴。
	LegacySaturatedAxes = 4
)

// AxesAt100 は、100 点の軸の数。
func AxesAt100(p *models.CompanyWeightProfile) int {
	n := 0
	for _, a := range profileAxes(p) {
		if *a >= 100 {
			n++
		}
	}
	return n
}

// IsLegacySaturated は、作り直しの候補（100 に張り付いた旧い生成のプロファイル）か。
func IsLegacySaturated(p *models.CompanyWeightProfile) bool {
	return p != nil && AxesAt100(p) >= LegacySaturatedAxes
}

// ProfileSpread は、軸ごとの企業間の幅（標準偏差）と、100 点が多い企業の数。作り直しの前後を比べる。
type ProfileSpread struct {
	Companies int
	// AxisStd は 10 軸の企業間の標準偏差（profileAxes と同じ並び）。
	AxisStd [10]float64
	// Informative は、標準偏差が 15 以上の軸の数（matching.MinAxisSpread と同じ基準）。
	Informative int
	// Saturated は、100 点の軸が LegacySaturatedAxes 以上ある企業の数。
	Saturated int
}

// MeasureProfileSpread は、プロファイルの集まりの、企業間の幅を測る。
func MeasureProfileSpread(profiles []*models.CompanyWeightProfile) ProfileSpread {
	s := ProfileSpread{Companies: len(profiles)}
	if len(profiles) == 0 {
		return s
	}
	var sum, sumSq [10]float64
	for _, p := range profiles {
		if IsLegacySaturated(p) {
			s.Saturated++
		}
		for i, a := range profileAxes(p) {
			v := float64(*a)
			sum[i] += v
			sumSq[i] += v * v
		}
	}
	n := float64(len(profiles))
	for i := range s.AxisStd {
		mean := sum[i] / n
		s.AxisStd[i] = math.Sqrt(math.Max(0, sumSq[i]/n-mean*mean))
		if s.AxisStd[i] >= 15 {
			s.Informative++
		}
	}
	return s
}
