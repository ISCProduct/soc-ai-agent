package sttbench

import "sort"

// GroupSummary は録音条件または由来ごとの集計（#1484）。
//
// 合成音声だけの結果を本番品質の根拠にしないため、条件別・由来別に
// 数値を分けて出す。母数が小さいときに全体平均へ埋もれるのを防ぐ。
type GroupSummary struct {
	Key   string `json:"key"`
	Cases int    `json:"cases"`
	// Errored は API 呼び出し自体が失敗した件数。どの平均の母数にも入らない。
	Errored         int     `json:"errored"`
	MeanCER         float64 `json:"mean_cer"`
	MeanSemanticCER float64 `json:"mean_semantic_cer"`
	KeywordAccuracy float64 `json:"keyword_accuracy"`
	NumberAccuracy  float64 `json:"number_accuracy"`
	// FailureRate は認識失敗率。分母は API が成功した件数（Cases - Errored）。
	FailureRate float64 `json:"failure_rate"`
}

// Breakdown は CaseResult 群を key ごとに集計する純関数。
//
// keyOf でグルーピングの軸を差し替える（条件別・由来別で使い回す）。
// キーワード・数値の正解率は、対象が無いグループでは 0 ではなく
// 「母数なし」を表す -1 を返す。0（全滅）と区別するため。
//
// 母数に入れないのは APIエラーだけ。呼べていないので何も測れていない。
//
// 認識失敗（IsRecognitionFailure）は**全ミスとして数える**。APIは正常応答して
// いて、モデルが空・短すぎる出力を返したという測定結果そのものだからである。
// CER 等の平均から外すと「何も返さないほど成績が良く見える」逆転が起きる
// （完璧1件＋無音1件で MeanCER=0.000 / KeywordAccuracy=1.000 になっていた）。
// run.go は r.Failed を立てる前に CER・キーワードを計算しているので、
// 空出力なら CER=1.0 / KeywordsHit=0 が既に入っており、そのまま足せばよい。
func Breakdown(results []CaseResult, keyOf func(CaseResult) string) []GroupSummary {
	type acc struct {
		n               int
		cer, sem        float64
		kwHit, kwTot    int
		numHit, numTot  int
		failed, errored int
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
		if r.Errored {
			a.errored++
			continue
		}
		if r.Failed {
			// failure_rate の分子に数えるだけ。CER・キーワードの母数からは
			// 外さない（外すと失敗が多いほど good に見える）。
			a.failed++
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
		apiOK := a.n - a.errored // APIが成功した件数＝全指標の母数
		g := GroupSummary{Key: k, Cases: a.n, Errored: a.errored}
		if apiOK > 0 {
			g.MeanCER = a.cer / float64(apiOK)
			g.MeanSemanticCER = a.sem / float64(apiOK)
		}
		if apiOK > 0 {
			g.FailureRate = float64(a.failed) / float64(apiOK)
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
