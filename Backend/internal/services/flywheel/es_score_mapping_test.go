package flywheel

import "testing"

// ES添削の写像はマッチングに直接効くので、保存値を数値で固定する（#1531）。
// 式: 素点を 1〜10 → 0〜100 に移し、50 + (素点 - 50) * weight。
// コミュニケーション力は (specificity + star) / 2 を weight 0.8、
// 細部志向は length_balance を weight 0.6。
func TestMapESScore_SavedValues(t *testing.T) {
	tests := []struct {
		name                             string
		specificity, star, lengthBalance int
		want                             map[string]int
	}{
		{
			// 素点 5.5 相当（1〜10の中央）はどのカテゴリも中立50のまま動かない。
			// ここがずれていると、添削するだけで全員のスコアが一方向に動く。
			name: "中央の評価は中立のまま", specificity: 6, star: 5, lengthBalance: 6,
			// (6+5)/2=5 → 44、50+(44-50)*0.8=45.2→45 / 6 → 55、50+(55-50)*0.6=53
			want: map[string]int{"コミュニケーション力": 45, "細部志向": 53},
		},
		{
			name: "最高評価", specificity: 10, star: 10, lengthBalance: 10,
			// 10 → 100、50+50*0.8=90 / 50+50*0.6=80
			want: map[string]int{"コミュニケーション力": 90, "細部志向": 80},
		},
		{
			name: "最低評価", specificity: 1, star: 1, lengthBalance: 1,
			// 1 → 0、50-50*0.8=10 / 50-50*0.6=20
			// 低い評価がきちんと低く出ること（上方バイアスを作らない）。
			want: map[string]int{"コミュニケーション力": 10, "細部志向": 20},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := map[string]int{}
			for _, c := range mapESScore(tt.specificity, tt.star, tt.lengthBalance) {
				got[c.category] = c.score
			}
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
	got := map[string]bool{}
	for _, c := range mapESScore(8, 8, 8) {
		got[c.category] = true
	}
	for _, forbidden := range []string{"技術志向", "チームワーク", "リーダーシップ", "創造性", "安定志向", "成長志向", "ワークライフバランス", "チャレンジ精神"} {
		if got[forbidden] {
			t.Errorf("%s には反映しないはず", forbidden)
		}
	}
	if !got["コミュニケーション力"] || !got["細部志向"] {
		t.Fatalf("反映すべきカテゴリが欠けている: %v", got)
	}
}

// 1〜10 の範囲外が来ても丸めて写す。RAG 側の異常で範囲外が来たときに
// 0〜100 を外れた値をリポジトリへ渡さない（#1528 と同じ理由）。
func TestEsRawToHundred_ClampsOutOfRange(t *testing.T) {
	if got := esRawToHundred(0); got != 0 {
		t.Errorf("下限は0: %d", got)
	}
	if got := esRawToHundred(99); got != 100 {
		t.Errorf("上限は100: %d", got)
	}
}
