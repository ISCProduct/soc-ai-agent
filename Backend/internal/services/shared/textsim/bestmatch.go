package textsim

// BestMatch は needle が h のどこかに現れる度合い（0.0〜1.0）を返す（#1527）。
//
// MatchScore との違いは、h が長くても短い引用を拾えること。
// MatchScore は h 全体と1回比べるので、完全な引用（部分文字列）は 1.0 で拾えるが、
// 要約や言い換えは h が長いほど Dice の分母が膨らんで 0 に近づく。
// 面接1回の発話を丸ごと h にすると、正しい要約でもほぼ 0 になってしまう。
//
// そこで needle と同じ幅（および2倍幅）の窓を1文字ずつすべらせ、窓ごとの
// MatchScore の最大値を採る。2倍幅も見るのは、要約元のスパンが needle より
// 長いケースを拾うため。
//
// needle が正規化後に空（記号や絵文字だけ）なら 0.0。h が空でも 0.0。
//
// ## この値を「実在する引用か」の判定にそのまま使わないこと
//
// 窓が日本語の述語末尾（「〜することができました」等）に重なるだけで 0.27〜0.56 に
// 乗るため、**内容語がすべて捏造でも高い値が出る**。
// フィラー（「はい」等）は部分文字列なので 1.0 になる。
// しきい値をどこに置いてもこれらは分離できない（Issue #1566、
// 実測は docs/wiki/scoring.md §2-4）。
//
// ponytail: 窓を1文字ずつ総当たりする O(len(h)×len(needle)) の素朴な実装。
// 面接1回の発話（数千文字）× 引用10件程度なら数十msで済む。文書単位に広げるなら
// bigram の出現位置を先にインデックス化して候補窓を絞ること。
func BestMatch(needle string, h Bigrams) float64 {
	n := New(needle)
	if n.Empty() || h.Empty() {
		return 0
	}
	best := h.MatchScore(n)
	// 窓の切り出しにだけ rune 列が要る。Bigrams に持たせず都度作る
	// （窓ループのコストが支配的で、1回の変換は誤差）。
	hr := []rune(h.norm)
	for _, width := range []int{n.Len(), n.Len() * 2} {
		if best == 1 {
			break
		}
		if width >= len(hr) {
			// 窓が h 以上なら全体と比べたのと同じ。上の MatchScore で済んでいる
			continue
		}
		for i := 0; i+width <= len(hr); i++ {
			// 窓は h.norm の部分文字列なので正規化済み。New に通しても no-op
			best = max(best, New(string(hr[i:i+width])).MatchScore(n))
		}
	}
	return best
}
