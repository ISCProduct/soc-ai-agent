package flywheel

import "testing"

func esScores(specificity, star, lengthBalance int) map[string]int {
	out := map[string]int{}
	for _, c := range mapESScore(specificity, star, lengthBalance) {
		out[c.category] = c.score
	}
	return out
}

// 中立の入力は中立のまま出る（#1531）。
//
// これは「値がこうなる」ではなく守るべき性質。1〜10 の中央は 5.5 なので、
// (6,5) や (5,6) はちょうど中立50にならなければならない。
// 1〜10 の整数空間で平均すると (6+5)/2 が 5 に切り捨てられて 45 になり、
// 中位の添削を繰り返すだけで全員のスコアが下へ寄る（マッチングの一方向の歪み）。
// この性質を落とすと、書き込みが既存セッションへの加算なので後から戻せない。
func TestMapESScore_NeutralInputStaysNeutral(t *testing.T) {
	for _, p := range [][2]int{{6, 5}, {5, 6}} {
		got := esScores(p[0], p[1], 5)
		if got["コミュニケーション力"] != neutralScore {
			t.Errorf("specificity=%d star=%d のコミュニケーション力 = %d, want %d（中立）",
				p[0], p[1], got["コミュニケーション力"], neutralScore)
		}
	}
}

// 写像は中立50を基準に上下対称。最高評価と最低評価が中立から等距離に出る。
// 非対称だと、添削するだけでスコアが一方向へ動く。
func TestMapESScore_SymmetricAroundNeutral(t *testing.T) {
	high := esScores(10, 10, 10)
	low := esScores(1, 1, 1)
	for _, cat := range []string{"コミュニケーション力", "細部志向"} {
		up := high[cat] - neutralScore
		down := neutralScore - low[cat]
		if up != down {
			t.Errorf("%s が中立から非対称: 上 %d / 下 %d", cat, up, down)
		}
	}
}

// 保存値を数値で固定する。weight を変えると落ちる。
// コミュニケーション力は (具体性 + STAR) を 0〜100 に移してから平均し weight 0.8、
// 細部志向は文字数バランスを 0〜100 に移して weight 0.6。
func TestMapESScore_SavedValues(t *testing.T) {
	tests := []struct {
		name                             string
		specificity, star, lengthBalance int
		want                             map[string]int
	}{
		// 10 → 100。50+50*0.8=90 / 50+50*0.6=80
		{"最高評価", 10, 10, 10, map[string]int{"コミュニケーション力": 90, "細部志向": 80}},
		// 1 → 0。50-50*0.8=10 / 50-50*0.6=20
		{"最低評価", 1, 1, 1, map[string]int{"コミュニケーション力": 10, "細部志向": 20}},
		// (7,6) → (66.67+55.56)/2=61.11、50+11.11*0.8=58.9→59 / 5 → 44.44、50-5.56*0.6=46.7→47
		{"中位より少し上", 7, 6, 5, map[string]int{"コミュニケーション力": 59, "細部志向": 47}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := esScores(tt.specificity, tt.star, tt.lengthBalance)
			if len(got) != len(tt.want) {
				t.Fatalf("反映するカテゴリ数が違う: %v", got)
			}
			for cat, want := range tt.want {
				if got[cat] != want {
					t.Errorf("%s = %d, want %d", cat, got[cat], want)
				}
			}
		})
	}
}

// 説明できないカテゴリには載せない（#1528 と同じ方針）。
// 企業適合性は企業名を指定した添削でしか付かないため、載せると
// 企業名を入れたかどうかでマッチングが変わってしまう。
func TestMapESScore_OnlyExplainableCategories(t *testing.T) {
	got := esScores(8, 8, 8)
	for _, forbidden := range []string{"技術志向", "チームワーク", "リーダーシップ", "創造性", "安定志向", "成長志向", "ワークライフバランス", "チャレンジ精神"} {
		if _, ok := got[forbidden]; ok {
			t.Errorf("%s には反映しないはず", forbidden)
		}
	}
	if _, ok := got["コミュニケーション力"]; !ok {
		t.Error("コミュニケーション力が欠けている")
	}
	if _, ok := got["細部志向"]; !ok {
		t.Error("細部志向が欠けている")
	}
}

// 1〜10 の範囲外が来ても丸めて写す。RAG 側の異常で範囲外が来たときに
// 0〜100 を外れた値をリポジトリへ渡さない（#1528 と同じ理由）。
func TestEsRawToHundred_ClampsOutOfRange(t *testing.T) {
	if got := esRawToHundred(0); got != 0 {
		t.Errorf("下限は0: %v", got)
	}
	if got := esRawToHundred(99); got != 100 {
		t.Errorf("上限は100: %v", got)
	}
	// 中央の 5.5 が中立50になる（整数で割ると崩れる性質）。
	if mid := (esRawToHundred(5) + esRawToHundred(6)) / 2; mid != float64(neutralScore) {
		t.Errorf("5と6の中央が中立になっていない: %v", mid)
	}
}
