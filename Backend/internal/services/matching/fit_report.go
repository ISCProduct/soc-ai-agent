package matching

import (
	"math"
	"sort"

	"Backend/internal/models"
)

// AxisDistribution は、1 軸の企業間の分布。
type AxisDistribution struct {
	Name        string
	Mean, Std   float64
	Median      float64
	ShareAt100  float64 // 100 点の企業の割合
	Informative bool    // 企業間でばらつきのある軸（InformativeAxes）
}

// FitReport は、ずれる軸の効果を集計だけで示す（個人・企業を識別できる値は含まない）。
type FitReport struct {
	Companies         int
	Axes              [10]AxisDistribution
	CompaniesGE6At100 int // 10 軸のうち 6 軸以上が 100 点の企業
	Sessions          int
	// 上位 N 件に占める割合（セッション平均）と、全企業のうち参考になる割合
	TopOff1, TopOn1, TopOffRef, TopOnRef, TopShift, AllRef, AllRefMax float64
}

// BuildFitReport は、公開企業のプロファイルと、評価済みのセッション（軸名→点）から、分布と効果を集計する。
// 減点なしの得点は「減点後 + 減点幅」で求める（0 で打ち切られた分は無視できる）。
func BuildFitReport(profiles []*models.CompanyWeightProfile, sessions []map[string]float64, topN int) FitReport {
	r := FitReport{Companies: len(profiles), Sessions: len(sessions)}
	inf := InformativeAxes(profiles)
	n := float64(len(profiles))
	for i := range r.Axes {
		vals := make([]float64, 0, len(profiles))
		at100 := 0
		var sum, sumSq float64
		for _, p := range profiles {
			v := profileAxisValues(p)[i]
			vals = append(vals, v)
			sum += v
			sumSq += v * v
			if v >= 100 {
				at100++
			}
		}
		sort.Float64s(vals)
		d := AxisDistribution{Name: axisNames[i], Informative: inf[i]}
		if n > 0 {
			d.Mean = sum / n
			d.Std = math.Sqrt(math.Max(0, sumSq/n-d.Mean*d.Mean))
			d.Median = vals[len(vals)/2]
			d.ShareAt100 = float64(at100) / n
		}
		r.Axes[i] = d
	}
	for _, p := range profiles {
		c := 0
		for _, v := range profileAxisValues(p) {
			if v >= 100 {
				c++
			}
		}
		if c >= 6 {
			r.CompaniesGE6At100++
		}
	}
	if len(profiles) == 0 || len(sessions) == 0 || topN <= 0 {
		return r
	}

	svc := &MatchingService{}
	for _, u := range sessions {
		type row struct {
			id        int
			on, off   float64
			mismatchN int
		}
		rows := make([]row, len(profiles))
		for i, p := range profiles {
			m := svc.calculateMatchScoreFor(u, p, nil, &inf)
			k := len(mismatchAxisList(m))
			rows[i] = row{id: i, on: m.MatchScore, off: m.MatchScore + FitPenalty(k), mismatchN: k}
			if !fitPenaltyEnabled() {
				rows[i].off = m.MatchScore
			}
		}
		top := func(less func(a, b row) bool) []row {
			cp := append([]row{}, rows...)
			sort.SliceStable(cp, func(a, b int) bool { return less(cp[a], cp[b]) })
			if len(cp) > topN {
				cp = cp[:topN]
			}
			return cp
		}
		off := top(func(a, b row) bool { return a.off > b.off })
		on := top(func(a, b row) bool { return a.on > b.on })
		share := func(rs []row, min int) float64 {
			c := 0
			for _, x := range rs {
				if x.mismatchN >= min {
					c++
				}
			}
			return float64(c) / float64(len(rs))
		}
		r.TopOff1 += share(off, 1)
		r.TopOn1 += share(on, 1)
		r.TopOffRef += share(off, ReferenceMismatchAxes)
		r.TopOnRef += share(on, ReferenceMismatchAxes)
		all := share(rows, ReferenceMismatchAxes)
		r.AllRef += all
		r.AllRefMax = math.Max(r.AllRefMax, all)
		set := map[int]bool{}
		for _, x := range off {
			set[x.id] = true
		}
		same := 0
		for _, x := range on {
			if set[x.id] {
				same++
			}
		}
		r.TopShift += 1 - float64(same)/float64(len(on))
	}
	s := float64(len(sessions))
	r.TopOff1, r.TopOn1, r.TopOffRef, r.TopOnRef, r.TopShift, r.AllRef = r.TopOff1/s, r.TopOn1/s, r.TopOffRef/s, r.TopOnRef/s, r.TopShift/s, r.AllRef/s
	return r
}
