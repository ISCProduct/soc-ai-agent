package company

// 企業プロファイルに識別力を持たせるための正規化(#1331)。
//
// マッチ度は 100 - |学生スコア - 企業重視度| で、企業ごとの平均を取る。
// ある軸が全社で同じ値なら、その軸は全企業に同じ定数を足すだけになり、
// 並び順に一切寄与しない。実測では teamwork の標準偏差が 8.1、detail が 4.5 で、
// 90社中 40〜60 に入る企業が0社という状態だった（上限側に張り付いている）。
//
//	teamwork 平均99 / 標準偏差8.1
//	detail   平均99 / 標準偏差4.5
//
// 原因は生成プロンプトが各軸を独立した絶対評価で尋ねていたこと。
// 「この企業はチームワークを重視しますか」と個別に聞けば、どの企業でも
// 「重視する」と答えるのが自然で、結果として全社が上限に並ぶ。
//
// ここでは LLM が付けた「順序」は信頼し、「幅」だけを補正する。
// 順序を作り直すと存在しない情報を捏造することになるため、
// 線形変換で引き伸ばすだけに留める。

import "Backend/internal/models"

// minProfileSpread は10軸の最大値と最小値の差に求める最小の幅。
//
// これを下回るプロファイルは、どの企業とも似た形になり識別に使えない。
// 40 は「上位軸と下位軸がマッチ度で40点分の差を生む」ことを意味する。
// 大きくしすぎると LLM が付けたわずかな差を過剰に増幅するため、
// 0〜100 の半分未満に収めている。
const minProfileSpread = 40

// profileAxes はプロファイルの10軸への参照を返す。
// 走査と書き戻しを1箇所にまとめないと、軸を足したときに片方だけ漏れる。
func profileAxes(p *models.CompanyWeightProfile) []*int {
	return []*int{
		&p.TechnicalOrientation,
		&p.TeamworkOrientation,
		&p.LeadershipOrientation,
		&p.CreativityOrientation,
		&p.StabilityOrientation,
		&p.GrowthOrientation,
		&p.WorkLifeBalance,
		&p.ChallengeSeeking,
		&p.DetailOrientation,
		&p.CommunicationSkill,
	}
}

// NormalizeProfileSpread は normalizeProfileSpread の公開版。
// 既存プロファイルを一括で直す cmd/amplify-profiles から使う。
func NormalizeProfileSpread(p *models.CompanyWeightProfile) bool {
	return normalizeProfileSpread(p)
}

// normalizeProfileSpread は10軸の幅が狭すぎる場合に、順序を保ったまま引き伸ばす。
//
// 全軸が同じ値のときは引き伸ばしようがないので何もしない。
// その場合は生成をやり直すしかなく、呼び出し元が判断する。
func normalizeProfileSpread(p *models.CompanyWeightProfile) bool {
	if p == nil {
		return false
	}
	axes := profileAxes(p)

	minV, maxV := *axes[0], *axes[0]
	for _, a := range axes {
		if *a < minV {
			minV = *a
		}
		if *a > maxV {
			maxV = *a
		}
	}

	spread := maxV - minV
	if spread >= minProfileSpread {
		return false // 既に識別力がある
	}
	if spread == 0 {
		// 全軸が同値。順序が無いので引き伸ばせない。
		return false
	}

	// [minV, maxV] を幅 minProfileSpread のウィンドウへ線形に写す。
	//
	// 中心を保ったまま倍率をかけるだけだと、上限側に寄ったプロファイルでは
	// 100 で切られて目標の幅に届かない（99付近の例で 5 → 23 にしかならない）。
	// 先にウィンドウを 0〜100 に収まる位置へずらしてから写す。
	center := float64(minV+maxV) / 2
	half := float64(minProfileSpread) / 2
	lo, hi := center-half, center+half
	if hi > 100 {
		lo -= hi - 100
		hi = 100
	}
	if lo < 0 {
		hi += -lo
		lo = 0
	}

	span := float64(maxV - minV)
	for _, a := range axes {
		ratio := (float64(*a) - float64(minV)) / span
		*a = clampScore(int(lo + ratio*(hi-lo) + 0.5))
	}
	return true
}

// profileIsDegenerate は識別に使えないプロファイルかを返す。
// 全軸が同値、または幅がほぼ無いものを指す。
func profileIsDegenerate(p *models.CompanyWeightProfile) bool {
	if p == nil {
		return true
	}
	axes := profileAxes(p)
	minV, maxV := *axes[0], *axes[0]
	for _, a := range axes {
		if *a < minV {
			minV = *a
		}
		if *a > maxV {
			maxV = *a
		}
	}
	// 10点未満の幅は、マッチ度の差が最大10点にしかならず順位をほぼ動かさない。
	return maxV-minV < 10
}
