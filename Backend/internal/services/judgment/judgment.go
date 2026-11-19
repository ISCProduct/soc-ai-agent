// Package judgment は、チャット診断が「判定できたか」を決める（SOCAIAGENT-668）。
//
// 以前は、フェーズごとの最低質問数（合計 15 問）で完了を決めていた。質問数を固定せず、次のように決める。
//   - 判定できたら、15 問に満たなくても完了にする。
//   - 15 問を超えても判定できなければ、判定できるまで質問を続ける。
//   - 終わらない質問に学生を閉じ込めないため、SoftCapAnswers を超えたら、暫定の結果を見る導線を出す
//     （質問は続けられる）。
//
// ここに置く閾値（FloorValidAnswers・MinScoreSpread など）は、現時点では経験則の初期値。
// 評価セットで較正する（SOCAIAGENT-670）。値を変えるときは、このファイルだけを直す。
package judgment

const (
	// FloorValidAnswers は、早まった完了を防ぐ床。有効な回答がこれ未満なら、ほかの条件を満たしても完了にしない。
	FloorValidAnswers = 6
	// MinEvaluatedCategories は、評価済みカテゴリ数の下限。
	//
	// 推薦を暫定表示にする下限（matching.MinEvaluatedCategoriesForFinal = 4）より大きくする。
	// 使う軸が少ないと、推薦の上位が、全軸で出した場合と大きく変わる（不安定になる）。
	// 軸の数を増やすほど安定するので、早すぎる完了を避けるため 6 にしている（質問は 2〜3 問増える）。
	// matching は chat を経由してこのパッケージに循環するので import できない。
	// 完了の下限が暫定表示の下限を下回らないことは、judgment_sync_test.go で守る。
	MinEvaluatedCategories = 6
	// MinScoreSpread は、評価済みカテゴリの最高点と最低点の差の下限（0〜100 の目盛り）。
	// 全部が同じような点だと、何に向いているかを言い切れない。
	MinScoreSpread = 20
	// SoftCapAnswers は、これ以上答えても判定できないときに、暫定の結果を見る導線を出す回答数の目安。
	SoftCapAnswers = 30
)

// Input は判定に使う、会話の状態。
type Input struct {
	// ValidAnswers は有効に答えた質問数（スキップ・無効な回答は含めない）。
	ValidAnswers int
	// Scores は、カテゴリごとのスコア（0〜100）。0 は未評価として数える。
	Scores map[string]int
}

// Result は判定の結果。
type Result struct {
	// Percent は「判定の進み具合」（0〜100）。Ready のときだけ 100 になる。
	Percent int
	// Ready は、判定できた（完了にしてよい）か。
	Ready bool
	// 各条件の達成度（0〜1）。画面には出さず、原因の調査とテストに使う。
	Floor, Coverage, Spread float64
}

// Evaluate は、会話の状態から判定の進み具合を計算する。
func Evaluate(in Input) Result {
	evaluated, lo, hi := 0, 101, -1
	for _, s := range in.Scores {
		if s == 0 {
			continue
		}
		evaluated++
		if s < lo {
			lo = s
		}
		if s > hi {
			hi = s
		}
	}
	spread := 0
	if evaluated >= 2 {
		spread = hi - lo
	}

	r := Result{
		Floor:    ratio(in.ValidAnswers, FloorValidAnswers),
		Coverage: ratio(evaluated, MinEvaluatedCategories),
		Spread:   ratio(spread, MinScoreSpread),
	}

	// 職種が決まっているかは、完了の条件にしない。「まだ決めていない」人も、適性（スコア）の傾向が十分に出れば、
	// その傾向から向いているところを提案できる（推薦はスコアで計算する）。
	r.Ready = r.Floor >= 1 && r.Coverage >= 1 && r.Spread >= 1
	mean := (r.Floor + r.Coverage + r.Spread) / 3
	r.Percent = int(mean * 100)
	if !r.Ready && r.Percent > 99 {
		r.Percent = 99
	}
	if r.Ready {
		r.Percent = 100
	}
	return r
}

// SuggestProvisional は、暫定の結果を見る導線を出すか。判定できていなくて、目安の回答数を超えたとき。
func SuggestProvisional(answered int, ready bool) bool {
	return !ready && answered >= SoftCapAnswers
}

func ratio(n, d int) float64 {
	if n <= 0 {
		return 0
	}
	if n >= d {
		return 1
	}
	return float64(n) / float64(d)
}
