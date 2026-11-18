package matching

import (
	"math"
	"os"
	"strings"

	"Backend/internal/models"
)

// 適合度の「ずれ」の扱い（SOCAIAGENT-670）。
//
// 適合度は各軸の近さ（100 − 差）の平均なので、ある軸で大きく離れていても、ほかの軸で
// 相殺されて上位に残る。本人の傾向と企業の重視点が大きく離れた軸（ずれる軸）がある企業は、
// その数だけ適合度を下げる。2軸以上ずれるものは、推薦の一覧で「参考」として理由を添える。
//
// 企業間でほとんど値が変わらない軸（全社が同じ高さなど）は、ずれても企業を区別する根拠にならない。
// その軸でのずれは全社に同じ減点が入るだけで、並びは変わらず、全社に「参考」が付いてしまう。
// そのため、ずれる軸は「企業間でばらつきのある軸」だけで数える（InformativeAxes）。
//
// 閾値は初期値。評価（fit_eval_test.go）と、実データ（開発DB）の集計で決めた。
const (
	// MismatchAxisFloor は、軸ごとのマッチ度（100 − 差）がこれ未満なら「ずれる軸」。差が 50 を超えた軸。
	MismatchAxisFloor = 50.0
	// MismatchPenaltyPerAxis は、ずれる軸 1 つにつき下げる適合度。
	MismatchPenaltyPerAxis = 6.0
	// MismatchPenaltyMax は、下げる幅の上限。
	MismatchPenaltyMax = 18.0
	// ReferenceMismatchAxes は、この数以上の軸でずれたら「参考」の扱いにする。
	ReferenceMismatchAxes = 2
	// MinAxisSpread は、企業間の標準偏差がこれ以上の軸を「ばらつきのある軸」とする。
	MinAxisSpread = 15.0
	// MinCompaniesForSpread は、ばらつきを測る最小の企業数。これ未満は、比べる根拠が無いのでずれを数えない。
	MinCompaniesForSpread = 5
)

// axisNames は 10 軸の名前。並びは profileAxisValues / axisMatchesOf と同じ。
var axisNames = [10]string{"技術志向", "チームワーク志向", "リーダーシップ志向", "創造性志向", "安定志向", "成長志向", "ワークライフバランス", "チャレンジ志向", "細部志向", "コミュニケーション力"}

// AxisFlags は 10 軸ごとの真偽（axisNames と同じ並び）。
type AxisFlags [10]bool

// fitPenaltyEnabled は、ずれの減点を使うか。MATCHING_FIT_PENALTY=off で切り戻せる。
func fitPenaltyEnabled() bool { return os.Getenv("MATCHING_FIT_PENALTY") != "off" }

func profileAxisValues(p *models.CompanyWeightProfile) [10]float64 {
	return [10]float64{
		float64(p.TechnicalOrientation), float64(p.TeamworkOrientation), float64(p.LeadershipOrientation),
		float64(p.CreativityOrientation), float64(p.StabilityOrientation), float64(p.GrowthOrientation),
		float64(p.WorkLifeBalance), float64(p.ChallengeSeeking), float64(p.DetailOrientation), float64(p.CommunicationSkill),
	}
}

// InformativeAxes は、企業間でばらつきのある軸（標準偏差が MinAxisSpread 以上）。
// 企業が MinCompaniesForSpread 社未満なら、どの軸も「ばらつきのある軸」としない（比べる根拠が無い）。
func InformativeAxes(profiles []*models.CompanyWeightProfile) AxisFlags {
	var out AxisFlags
	if len(profiles) < MinCompaniesForSpread {
		return out
	}
	var sum, sumSq [10]float64
	for _, p := range profiles {
		for i, v := range profileAxisValues(p) {
			sum[i] += v
			sumSq[i] += v * v
		}
	}
	n := float64(len(profiles))
	for i := range out {
		mean := sum[i] / n
		out[i] = math.Sqrt(math.Max(0, sumSq[i]/n-mean*mean)) >= MinAxisSpread
	}
	return out
}

// MismatchAxes は、計測できた軸のうち、マッチ度が MismatchAxisFloor 未満で、ばらつきのある軸の名前。
// informative が nil のときは、全軸を対象にする。0 は「計測できなかった軸」なので数えない。
func MismatchAxes(axisMatches [10]float64, informative *AxisFlags) []string {
	var out []string
	for i, m := range axisMatches {
		if m <= 0 || m >= MismatchAxisFloor {
			continue
		}
		if informative != nil && !informative[i] {
			continue
		}
		out = append(out, axisNames[i])
	}
	return out
}

// FitPenalty は、ずれる軸の数に応じて下げる適合度。
func FitPenalty(mismatchAxes int) float64 {
	p := float64(mismatchAxes) * MismatchPenaltyPerAxis
	if p > MismatchPenaltyMax {
		return MismatchPenaltyMax
	}
	return p
}

// mismatchAxisList は、保存したずれる軸（カンマ区切り）を配列にする。
func mismatchAxisList(m *models.UserCompanyMatch) []string {
	if m.FitMismatchAxes == "" {
		return nil
	}
	return strings.Split(m.FitMismatchAxes, ",")
}

// FitNote は、「参考」の理由。ずれる軸が ReferenceMismatchAxes 未満なら、参考ではないので空文字。
func FitNote(m *models.UserCompanyMatch) string {
	axes := mismatchAxisList(m)
	if len(axes) < ReferenceMismatchAxes {
		return ""
	}
	return "参考: " + strings.Join(axes, "・") + "で、あなたの傾向と企業の重視点が大きく離れています。"
}

func axisMatchesOf(m *models.UserCompanyMatch) [10]float64 {
	return [10]float64{
		m.TechnicalMatch, m.TeamworkMatch, m.LeadershipMatch, m.CreativityMatch, m.StabilityMatch,
		m.GrowthMatch, m.WorkLifeMatch, m.ChallengeMatch, m.DetailMatch, m.CommunicationMatch,
	}
}
