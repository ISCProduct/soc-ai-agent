package matching

import (
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
// 閾値は初期値。評価（fit_eval_test.go）で、ずれた提案の割合を測って決めた。
const (
	// MismatchAxisFloor は、軸ごとのマッチ度（100 − 差）がこれ未満なら「ずれる軸」。差が 50 を超えた軸。
	MismatchAxisFloor = 50.0
	// MismatchPenaltyPerAxis は、ずれる軸 1 つにつき下げる適合度。
	MismatchPenaltyPerAxis = 6.0
	// MismatchPenaltyMax は、下げる幅の上限。
	MismatchPenaltyMax = 18.0
	// ReferenceMismatchAxes は、この数以上の軸でずれたら「参考」の扱いにする。
	ReferenceMismatchAxes = 2
)

// fitPenaltyEnabled は、ずれの減点を使うか。MATCHING_FIT_PENALTY=off で切り戻せる。
func fitPenaltyEnabled() bool { return os.Getenv("MATCHING_FIT_PENALTY") != "off" }

// MismatchAxisCount は、計測できた軸のうち、マッチ度が MismatchAxisFloor 未満の軸の数。
// 0 は「計測できなかった軸」なので数えない（本人が 0 点の軸は未評価として扱う）。
func MismatchAxisCount(axisMatches []float64) int {
	n := 0
	for _, m := range axisMatches {
		if m > 0 && m < MismatchAxisFloor {
			n++
		}
	}
	return n
}

// FitPenalty は、ずれる軸の数に応じて下げる適合度。
func FitPenalty(mismatchAxes int) float64 {
	p := float64(mismatchAxes) * MismatchPenaltyPerAxis
	if p > MismatchPenaltyMax {
		return MismatchPenaltyMax
	}
	return p
}

// mismatchAxisNames は、ずれる軸の名前（10 軸の並びは axisMatchesOf と同じ）。
func mismatchAxisNames(m *models.UserCompanyMatch) []string {
	names := []string{"技術志向", "チームワーク志向", "リーダーシップ志向", "創造性志向", "安定志向", "成長志向", "ワークライフバランス", "チャレンジ志向", "細部志向", "コミュニケーション力"}
	var out []string
	for i, v := range axisMatchesOf(m) {
		if v > 0 && v < MismatchAxisFloor {
			out = append(out, names[i])
		}
	}
	return out
}

// FitNote は、「参考」の理由。ずれる軸が ReferenceMismatchAxes 未満なら、参考ではないので空文字。
func FitNote(m *models.UserCompanyMatch) string {
	axes := mismatchAxisNames(m)
	if len(axes) < ReferenceMismatchAxes {
		return ""
	}
	return "参考: " + strings.Join(axes, "・") + "で、あなたの傾向と企業の重視点が大きく離れています。"
}

// axisMatchesOf は、マッチ度の 10 軸をまとめる（CategoryScores と同じ並び）。
func axisMatchesOf(m *models.UserCompanyMatch) []float64 {
	return []float64{
		m.TechnicalMatch, m.TeamworkMatch, m.LeadershipMatch, m.CreativityMatch, m.StabilityMatch,
		m.GrowthMatch, m.WorkLifeMatch, m.ChallengeMatch, m.DetailMatch, m.CommunicationMatch,
	}
}
