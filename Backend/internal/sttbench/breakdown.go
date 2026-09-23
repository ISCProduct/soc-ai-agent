package sttbench

import "sort"

// GroupSummary は録音条件または由来ごとの集計（#1484）。
//
// 合成音声だけの結果を本番品質の根拠にしないため、条件別・由来別に
// 数値を分けて出す。母数が小さいときに全体平均へ埋もれるのを防ぐ。
type GroupSummary struct {
	Key             string  `json:"key"`
	Cases           int     `json:"cases"`
	MeanCER         float64 `json:"mean_cer"`
	MeanSemanticCER float64 `json:"mean_semantic_cer"`
	KeywordAccuracy float64 `json:"keyword_accuracy"`
	NumberAccuracy  float64 `json:"number_accuracy"`
	FailureRate     float64 `json:"failure_rate"`
}

// Breakdown は CaseResult 群を key ごとに集計する純関数。
//
// keyOf でグルーピングの軸を差し替える（条件別・由来別で使い回す）。
// キーワード・数値の正解率は、対象が無いグループでは 0 ではなく
// 「母数なし」を表す -1 を返す。0（全滅）と区別するため。
func Breakdown(results []CaseResult, keyOf func(CaseResult) string) []GroupSummary {
	type acc struct {
		n               int
		cer, sem        float64
		kwHit, kwTot    int
		numHit, numTot  int
		failed          int
		kwSeen, numSeen bool
	}
	groups := map[string]*acc{}
	order := []string{}
	for _, r := range results {
		k := keyOf(r)
		a, ok := groups[k]
		if !ok {
			a = &acc{}
			groups[k] = a
			order = append(order, k)
		}
		a.n++
		if r.Failed {
			a.failed++
			// 失敗ケースは CER 等が意味を持たないので平均へ入れない。
			// ただし failure_rate の母数には数える。
			continue
		}
		a.cer += r.CER
		a.sem += r.SemanticCER
		a.kwHit += len(r.KeywordsHit)
		a.kwTot += len(r.KeywordsHit) + len(r.KeywordsMiss)
		if len(r.KeywordsHit)+len(r.KeywordsMiss) > 0 {
			a.kwSeen = true
		}
		a.numHit += r.NumbersMatched
		a.numTot += r.NumbersTotal
		if r.NumbersTotal > 0 {
			a.numSeen = true
		}
	}

	sort.Strings(order)
	out := make([]GroupSummary, 0, len(order))
	for _, k := range order {
		a := groups[k]
		scored := a.n - a.failed // 平均の母数は成功ケース数
		g := GroupSummary{Key: k, Cases: a.n}
		if scored > 0 {
			g.MeanCER = a.cer / float64(scored)
			g.MeanSemanticCER = a.sem / float64(scored)
		}
		if a.n > 0 {
			g.FailureRate = float64(a.failed) / float64(a.n)
		}
		g.KeywordAccuracy = ratioOrMissing(a.kwHit, a.kwTot, a.kwSeen)
		g.NumberAccuracy = ratioOrMissing(a.numHit, a.numTot, a.numSeen)
		out = append(out, g)
	}
	return out
}

// ratioOrMissing は母数があれば比率を、無ければ -1（母数なし）を返す。
// 0（全滅）と「対象が無い」を混同しないため。
func ratioOrMissing(hit, total int, seen bool) float64 {
	if !seen || total == 0 {
		return -1
	}
	return float64(hit) / float64(total)
}

// FilterByCondition は指定条件のケースだけを残す。空文字なら全件。
func FilterByCondition(cases []Case, condition string) []Case {
	if condition == "" {
		return cases
	}
	out := make([]Case, 0, len(cases))
	for _, c := range cases {
		if c.ConditionOf() == condition {
			out = append(out, c)
		}
	}
	return out
}
