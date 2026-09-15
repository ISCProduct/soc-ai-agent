package company

import (
	"Backend/internal/models"
	"math"
	"sort"
)

// EnsureProfileContrast は中央寄りのプロファイルにコントラストを付与する。
// 変更したら true。新規AI生成後・既存プロファイルの一括補正の両方で使う。
func EnsureProfileContrast(p *models.CompanyWeightProfile) bool {
	if p == nil || profileHasContrast(p) {
		return false
	}
	amplifyProfileContrast(p)
	if !profileHasContrast(p) {
		forceProfilePoles(p)
	}
	return true
}

// forceProfilePoles はストレッチでも足りないとき、最低2軸/最高2軸を押し出す。
func forceProfilePoles(p *models.CompanyWeightProfile) {
	type axis struct {
		get func() int
		set func(int)
	}
	axes := []axis{
		{func() int { return p.TechnicalOrientation }, func(v int) { p.TechnicalOrientation = v }},
		{func() int { return p.TeamworkOrientation }, func(v int) { p.TeamworkOrientation = v }},
		{func() int { return p.LeadershipOrientation }, func(v int) { p.LeadershipOrientation = v }},
		{func() int { return p.CreativityOrientation }, func(v int) { p.CreativityOrientation = v }},
		{func() int { return p.StabilityOrientation }, func(v int) { p.StabilityOrientation = v }},
		{func() int { return p.GrowthOrientation }, func(v int) { p.GrowthOrientation = v }},
		{func() int { return p.WorkLifeBalance }, func(v int) { p.WorkLifeBalance = v }},
		{func() int { return p.ChallengeSeeking }, func(v int) { p.ChallengeSeeking = v }},
		{func() int { return p.DetailOrientation }, func(v int) { p.DetailOrientation = v }},
		{func() int { return p.CommunicationSkill }, func(v int) { p.CommunicationSkill = v }},
	}
	order := make([]int, len(axes))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		return axes[order[i]].get() < axes[order[j]].get()
	})
	for _, idx := range order[:2] {
		if axes[idx].get() > 25 {
			axes[idx].set(25)
		}
	}
	for _, idx := range order[len(order)-2:] {
		if axes[idx].get() < 80 {
			axes[idx].set(80)
		}
	}
}

// profileHasContrast は企業が区別できるプロファイルかを見る。
// 中央寄り（多くが 40–60）だと線形マッチでも企業差が消える。
func profileHasContrast(p *models.CompanyWeightProfile) bool {
	if p == nil {
		return false
	}
	vals := profileAxisValues(p)
	low, high := 0, 0
	for _, v := range vals {
		if v <= 35 {
			low++
		}
		if v >= 70 {
			high++
		}
	}
	return low >= 2 && high >= 2
}

// amplifyProfileContrast は中立50からの距離を伸ばして企業差を出す。
// ponytail: 単純な線形ストレッチ。LLM再生成が安定したら置き換え可。
func amplifyProfileContrast(p *models.CompanyWeightProfile) {
	if p == nil {
		return
	}
	stretch := func(v int) int {
		return clampScore(int(math.Round(50 + float64(v-50)*2.0)))
	}
	p.TechnicalOrientation = stretch(p.TechnicalOrientation)
	p.TeamworkOrientation = stretch(p.TeamworkOrientation)
	p.LeadershipOrientation = stretch(p.LeadershipOrientation)
	p.CreativityOrientation = stretch(p.CreativityOrientation)
	p.StabilityOrientation = stretch(p.StabilityOrientation)
	p.GrowthOrientation = stretch(p.GrowthOrientation)
	p.WorkLifeBalance = stretch(p.WorkLifeBalance)
	p.ChallengeSeeking = stretch(p.ChallengeSeeking)
	p.DetailOrientation = stretch(p.DetailOrientation)
	p.CommunicationSkill = stretch(p.CommunicationSkill)
}

func profileAxisValues(p *models.CompanyWeightProfile) []int {
	return []int{
		p.TechnicalOrientation,
		p.TeamworkOrientation,
		p.LeadershipOrientation,
		p.CreativityOrientation,
		p.StabilityOrientation,
		p.GrowthOrientation,
		p.WorkLifeBalance,
		p.ChallengeSeeking,
		p.DetailOrientation,
		p.CommunicationSkill,
	}
}
