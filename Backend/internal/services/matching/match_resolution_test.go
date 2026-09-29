package matching

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"
)

// スコア1点の差がマッチング結果を動かすかの実測（#1560）。
//
// #1560 は「面接スコアの分解能が保存値に残らない」ことへの対応として
// user_weight_scores.score を DECIMAL 化する案を挙げていた。
// 小数を持つ価値があるかは、まず「1点の差が順位を変えるか」で決まる。
// ここはその判断根拠を再現可能な形で残すためのテストで、
// #1560 をもう一度開けるときは同じ数字から議論を始められるようにしてある。
//
// 結論（詳細は docs/wiki/scoring.md §6.3）:
//   - 1点は動かす。TestMatchScoreMovesOnePointPerUserPoint / TestOnePointChangesTopMatches。
//   - ただし int が刻む1点で既に動いているので、**小数を足す余地は無い**。
//     上位は同点が支配しており、同点の解消は DB の取得順という恣意的な順序に委ねられている。

// resolutionProfile はマッチ度計算に必要な10軸だけを持つ軽量プロファイル。
// 軸の順序は calculateMatchScore の呼び出し順と同じ。
type resolutionProfile [10]int

// matchScoreOf は calculateMatchScore と同じ式（計測できた軸の平均）でマッチ度を出す。
func matchScoreOf(user, company resolutionProfile) float64 {
	total := 0.0
	for i := range len(user) {
		total += CalculateCategoryMatch(float64(user[i]), float64(company[i]))
	}
	return total / float64(len(user))
}

// resolutionCompanies は LLM 生成 + profile_spread 正規化後の企業プロファイルを模した合成データ。
// 企業ごとに中心を振り、軸ごとに散らして整数へ丸める（実データの軸は整数）。
// 固定シードなので値は再現する。
func resolutionCompanies(n int, seed int64) []resolutionProfile {
	r := rand.New(rand.NewSource(seed))
	out := make([]resolutionProfile, n)
	for i := range n {
		center := 45 + r.Float64()*30 // 45〜75
		for a := range len(out[i]) {
			out[i][a] = clampAxis(int(math.Round(center + r.NormFloat64()*15)))
		}
	}
	return out
}

func clampAxis(v int) int {
	return max(0, min(100, v))
}

// rankByMatchScore はマッチ度降順の企業インデックスを返す。
// 並べ方は topMatchesByScore と同じ安定ソートで、同点は元の順序のまま残る。
func rankByMatchScore(user resolutionProfile, companies []resolutionProfile) []int {
	idx := make([]int, len(companies))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		return matchScoreOf(user, companies[idx[a]]) > matchScoreOf(user, companies[idx[b]])
	})
	return idx
}

// TestMatchScoreMovesOnePointPerUserPoint は
// 「ユーザースコア1点 = マッチ度 1/軸数 点」をちょうど満たすことを固定する。
//
// マッチ度が 100-|差| の線形で、総合が軸数の平均である限り、
// ユーザースコアも企業重視度も整数なので総合マッチ度は 1/軸数 の格子に乗る。
// この格子が #1560 の判断の土台（1点が動かす量が測れる）なので、
// CalculateCategoryMatch を非線形へ戻したら気付けるようにしておく。
func TestMatchScoreMovesOnePointPerUserPoint(t *testing.T) {
	const axes = 10
	step := 1.0 / axes

	user := resolutionProfile{50, 50, 50, 50, 50, 50, 50, 50, 50, 50}
	for _, company := range resolutionCompanies(200, 7) {
		base := matchScoreOf(user, company)
		// 総合マッチ度は 1/軸数 の格子に乗る
		if off := math.Abs(base*axes - math.Round(base*axes)); off > 1e-9 {
			t.Fatalf("マッチ度 %v が %v 刻みの格子から外れた", base, step)
		}
		// 1カテゴリを1点動かすと、ちょうど 1/軸数 だけ動く
		for _, delta := range []int{1, -1} {
			moved := user
			moved[0] = clampAxis(moved[0] + delta)
			if got := math.Abs(matchScoreOf(moved, company) - base); math.Abs(got-step) > 1e-9 {
				t.Fatalf("ユーザースコア%+d点でマッチ度が %v 動いた（%v のはず）", delta, got, step)
			}
		}
	}
}

// TestOnePointChangesTopMatches は1〜2点の差が上位10件の顔ぶれを変えることを固定する。
//
// #1560 の「1〜2点の差が実際に結果を変えるのか」への答えがこれ。
// 変わる。だから面接スコアの分解能そのものには意味がある。
// 一方で int が刻む1点で既に変わっているため、小数を足して得られるのは
// 同点の解消だけで、それは精度の改善ではない（同点は DB の取得順で割れている）。
func TestOnePointChangesTopMatches(t *testing.T) {
	// 面接レポートが書き込む軸（interviewScoreMapping の反映先）。
	// 技術志向 / チームワーク志向 / リーダーシップ志向 / 成長志向 /
	// チャレンジ志向 / 細部志向 / コミュニケーション力
	interviewAxes := []int{0, 1, 2, 5, 7, 8, 9}

	tests := []struct {
		name      string
		companies int
		seed      int64
		user      resolutionProfile
	}{
		{name: "本番相当の公開企業数", companies: 842, seed: 2,
			user: resolutionProfile{50, 50, 50, 50, 50, 50, 50, 50, 50, 50}},
		{name: "ばらけた学生", companies: 842, seed: 2,
			user: resolutionProfile{72, 48, 61, 55, 39, 66, 52, 58, 44, 69}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			companies := resolutionCompanies(tt.companies, tt.seed)
			// 表示されるのは GetTopMatches の既定10件。
			base := topMatchIDs(rankByMatchScore(tt.user, companies), 10)

			// 同点の多さを記録する。小数化しても「精度が上がる」のではなく
			// この同点が恣意的に割れるだけ、という判断の根拠になる。
			t.Logf("上位20件の隣接ギャップ: %s", adjacentGapHistogram(tt.user, companies, 20))

			for _, delta := range []int{1, 2} {
				// 面接が書く7軸をまとめて動かす（実際の反映の形）
				moved := tt.user
				for _, ax := range interviewAxes {
					moved[ax] = clampAxis(moved[ax] + delta)
				}
				changed := countNewEntrants(base, topMatchIDs(rankByMatchScore(moved, companies), 10))
				t.Logf("%+d点: 上位10件の入れ替わり %d件", delta, changed)
				if changed == 0 {
					t.Errorf("%+d点で上位10件が1件も入れ替わらなかった。"+
						"1点が結果を動かさないなら分解能を上げる意味が無く、#1560 の前提が変わる", delta)
				}
			}
		})
	}
}

func topMatchIDs(order []int, k int) map[int]bool {
	out := make(map[int]bool, k)
	for i := 0; i < k && i < len(order); i++ {
		out[order[i]] = true
	}
	return out
}

func countNewEntrants(before, after map[int]bool) int {
	n := 0
	for id := range after {
		if !before[id] {
			n++
		}
	}
	return n
}

// adjacentGapHistogram は上位 limit 件の隣接マッチ度差の内訳を返す。
func adjacentGapHistogram(user resolutionProfile, companies []resolutionProfile, limit int) string {
	scores := make([]float64, len(companies))
	for i, c := range companies {
		scores[i] = matchScoreOf(user, c)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(scores)))

	tie, one, more := 0, 0, 0
	for i := 1; i < limit && i < len(scores); i++ {
		switch gap := scores[i-1] - scores[i]; {
		case gap < 0.05:
			tie++
		case gap < 0.15:
			one++
		default:
			more++
		}
	}
	return fmt.Sprintf("同点 %d件 / 0.1点差 %d件 / 0.2点差以上 %d件", tie, one, more)
}
