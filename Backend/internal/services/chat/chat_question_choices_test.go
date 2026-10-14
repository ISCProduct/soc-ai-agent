package chat

import (
	"Backend/domain/valueobject"
	"strings"
	"testing"
)

// 評価軸は質問ごとに変わる。選択肢がフェーズ固定だったときは、
// フェーズが続くあいだ一字一句同じ選択肢が並び続けていた。
func TestBuildChoiceFallbackVariesByCategory(t *testing.T) {
	growth := buildChoiceFallback("新しいことに挑戦しましたか？", "成長志向")
	detail := buildChoiceFallback("細かいところまで気を配るタイプですか？", "細部志向")

	if growth == detail {
		t.Fatal("軸が違うのに選択肢が同一。質問が変わっても選択肢が更新されない")
	}
	for _, got := range []string{growth, detail} {
		if n := len(ParseChoiceOptions(got)); n != 5 {
			t.Fatalf("選択肢が5件でない: %d\n%s", n, got)
		}
		// 画面側はここを選択肢として読む。選択肢型でないと判定されるとボタンが出ない。
		if isTextBasedQuestion(got) {
			t.Fatalf("選択肢型として判定されない:\n%s", got)
		}
	}
}

// 採点は並び順そのもの（1番目=100 … 5番目=20）。
// 「その他」は ResolveChoiceAnswer が文章採点へ回すので末尾でよい。
func TestBuildChoiceFallbackIsOrdinalWithOtherLast(t *testing.T) {
	for _, c := range valueobject.AllWeightCategories() {
		category := string(c)
		question := buildChoiceFallback("この軸の質問です。", category)
		options := ParseChoiceOptions(question)
		if len(options) != 5 {
			t.Fatalf("%s: 選択肢が5件でない: %d", category, len(options))
		}

		last := options[len(options)-1]
		if !isOtherChoiceText(last.Text) {
			t.Fatalf("%s: 末尾が「その他」でない: %q", category, last.Text)
		}
		if got := ResolveChoiceAnswer(question, last.Value); !got.IsFreeText {
			t.Fatalf("%s: 「その他」が記号採点へ落ちている", category)
		}

		// 軸ごとの尺度が無いと一般尺度（とても当てはまる…）に落ち、
		// 全軸で同じ選択肢になってしまう。
		if strings.Contains(question, choiceScaleDefault[0]) {
			t.Fatalf("%s: 軸ごとの尺度が無く一般尺度に落ちている", category)
		}
		if first := ResolveChoiceAnswer(question, options[0].Value); first.Letter != "1" {
			t.Fatalf("%s: 先頭の選択肢が1番目として解決されない: %q", category, first.Letter)
		}
	}
}
