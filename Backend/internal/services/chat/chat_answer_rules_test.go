package chat

import "testing"

// isLikelyAnswer のルール表（answerRules）の、各ルールと順序の影響を固定する。
// 期待値は、ルール表にする前の実装と同じ結果であることを確かめたもの
// （15,000件超の入力で旧実装と突き合わせた）。
// ルールを足す・動かすときは、ここに例を足すこと。
func TestIsLikelyAnswer_Rules(t *testing.T) {
	const (
		episodeQ = "これまでの経験で印象に残っていることを具体的に教えてください。"
		jobQ     = "IT職種ではどれに興味がありますか？ 1) 開発 2) インフラ 3) データ"
	)
	tests := []struct {
		name     string
		answer   string
		question string
		want     bool
	}{
		// 無意味なパターンは先に弾く
		{"空", "", episodeQ, false},
		{"空白だけ", "　 ", episodeQ, false},
		{"同じ文字の連続", "あああああ", episodeQ, false},
		{"同じ文字の連続は長文でも優先して弾く", "あああああ、私は開発の経験があります", episodeQ, false},
		{"ほぼ記号のみ", "!!!!!!", episodeQ, false},
		{"日本語比率が低い英字の羅列", "abcdefgh", episodeQ, false},
		{"日本語比率が低くても IT 用語を含めば先へ進む", "Python Java", jobQ, true},

		// 長さ・選択肢・短答
		{"15文字超なら内容があるとみなす", "その経験は少ないですが児童養護施設の案件を担当しました", episodeQ, true},
		{"15文字超でも日本語比率が低ければ弾く", "abcdefghijklmnopq", episodeQ, false},
		{"選択肢の記号", "A", episodeQ, true},
		{"番号", "②", jobQ, true},
		{"はい", "はい", episodeQ, true},
		{"末尾の句読点は無視して短答を判定", "そうですね。", episodeQ, true},
		{"2文字未満", "あ", episodeQ, false},

		// 無回答は短答より先に判定する
		{"わからない", "わからない", episodeQ, false},
		{"分かりません（末尾の記号つき）", "分かりません…", episodeQ, false},

		// 雑談・羅列
		{"挨拶", "こんにちは", episodeQ, false},
		{"感謝", "ありがとうございます", episodeQ, false},
		{"キーワードの羅列は通常の質問では弾く", "りんご、みかん、ぶどう", episodeQ, false},
		{"職種選択の質問では単語の列挙を許容する", "開発 インフラ", jobQ, true},

		// 最後の砦
		{"5文字以上", "ほげほげほげ", episodeQ, true},
		{"IT 用語を含む短い回答", "AWS", episodeQ, true},
		{"短くて何にも当てはまらない", "ほげ", episodeQ, false},
		{"質問と共通する語がある短い回答", "経験", episodeQ, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLikelyAnswer(tt.answer, tt.question); got != tt.want {
				t.Fatalf("isLikelyAnswer(%q) = %v, want %v", tt.answer, got, tt.want)
			}
		})
	}
}

// ルール表の不変条件: reason が空・重複していないこと。ログと保守の手がかりになる。
func TestAnswerRules_ReasonsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i, r := range answerRules {
		if r.reason == "" || r.when == nil {
			t.Fatalf("answerRules[%d] に reason か when が無い", i)
		}
		if seen[r.reason] {
			t.Fatalf("reason %q が重複している", r.reason)
		}
		seen[r.reason] = true
	}
}
