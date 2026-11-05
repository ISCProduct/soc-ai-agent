package chat

import (
	"Backend/domain/valueobject"
	"Backend/internal/models"
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

// 「その他」を記号だけで選んだ回答は、内容が無いので通常の回答経路へ通さない。
// 通すと採点も進捗もできないまま次の質問だけ進み、フェーズが完了しなくなる。
func TestIsEmptyOtherChoiceAnswer(t *testing.T) {
	question := buildChoiceFallback("新しいことに挑戦しましたか？", "成長志向")
	cases := []struct {
		name   string
		answer string
		want   bool
	}{
		{"末尾のその他を記号だけで選ぶ", "5", true},
		{"全角空白つき", "　5　", true},
		{"通常の選択肢", "1", false},
		{"理由つきのその他", "5: 部活で新しい練習方法を考えました", false},
		{"自由記述", "資格の勉強を始めました", false},
		{"選択肢の無い質問", "5", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := question
			if tc.name == "選択肢の無い質問" {
				q = "最近頑張ったことを教えてください。"
			}
			if got := isEmptyOtherChoiceAnswer(q, tc.answer); got != tc.want {
				t.Fatalf("isEmptyOtherChoiceAnswer(%q)=%v want %v", tc.answer, got, tc.want)
			}
		})
	}
}

// 案内文は画面側が「警告」として読み飛ばせる必要がある。
// 読み飛ばせないと直近の質問として拾われ、選択肢が消えて選び直せなくなる。
func TestOtherNeedsTextMessageIsValidationFeedback(t *testing.T) {
	msg := validationOtherNeedsTextMessage()
	if !isValidationFeedbackMessage(msg) {
		t.Fatalf("案内文が警告として判定されない: %q", msg)
	}
	// 打ち切り扱いにはしない（「その他」を押しただけで終了されては困る）
	if strings.Contains(msg, validationMarkerTerminated) {
		t.Fatalf("案内文が打ち切りとして判定される: %q", msg)
	}
}

// 冒頭の案内（「…一番近いものを選んでください。」）と、選択肢に入る「役割」「興味」
// といった普通の語が重なると、本文のキーワード判定は職種選択の質問に倒れる。
// 倒れると採点が丸ごと飛ばされ、その軸が未評価のまま残る。
// 出題時に保存した評価軸があるときは、そちらを信じて採点を続ける。
func TestStoredCategoryBeatsJobSelectionHeuristic(t *testing.T) {
	s := &ChatService{}
	for _, c := range valueobject.AllWeightCategories() {
		category := string(c)
		question := buildChoiceFallback("あなたはどうですか？", category)
		history := []models.ChatMessage{
			{Role: "assistant", Content: question, WeightCategory: category},
		}

		// 職種判定へ再突入すると、回答が職種の候補として読み直される
		if s.shouldValidateJobCategory(history) {
			t.Fatalf("%s: 職種判定へ再突入する", category)
		}
		if got := findLastAssistantQuestionMessage(history); got == nil || got.WeightCategory != category {
			t.Fatalf("%s: 直近の質問から軸を取り出せない", category)
		}
	}
}

// 軸を持たない質問（職種選択）はこれまでどおり本文で判定する。
func TestJobSelectionWithoutStoredCategoryStillDetected(t *testing.T) {
	s := &ChatService{}
	history := []models.ChatMessage{
		{Role: "assistant", Content: "「開発系エンジニア」とのことですが、以下のうちどれが一番近いですか？\n1. ソフトウェアエンジニア\n2. Webエンジニア"},
	}
	if !s.shouldValidateJobCategory(history) {
		t.Fatal("職種選択の質問が検出されない")
	}
}

// 「その他」の案内を挟んだあとの再回答は、案内文ではなく元の質問を採点する。
// 案内文は軸を持たないので、拾うと軸が既定（技術志向）へ落ち、
// 成長志向の質問に答えたのに技術志向へ点が入る。
func TestScoringSkipsGuidanceAndKeepsOriginalQuestion(t *testing.T) {
	question := buildChoiceFallback("新しいことに挑戦しましたか？", "成長志向")
	history := []models.ChatMessage{
		{Role: "assistant", Content: question, WeightCategory: "成長志向"},
		{Role: "user", Content: "5"},
		{Role: "assistant", Content: validationOtherNeedsTextMessage()},
	}

	gotQuestion, gotCategory := lastQuestionForScoring(history)
	if gotCategory != "成長志向" {
		t.Fatalf("軸が引き継がれない: %q", gotCategory)
	}
	if gotQuestion != question {
		t.Fatalf("採点対象が元の質問でない:\n%s", gotQuestion)
	}

	// 無効回答の警告でも同じ（こちらは以前から保存されている文言）
	history[2].Content = validationRetryMessage(1, 3)
	if _, c := lastQuestionForScoring(history); c != "成長志向" {
		t.Fatalf("警告を挟むと軸が落ちる: %q", c)
	}
}

// 質問として判定できない履歴では従来どおり直近の assistant メッセージに落ちる。
func TestLastQuestionForScoringFallsBackToLatestAssistant(t *testing.T) {
	history := []models.ChatMessage{
		{Role: "assistant", Content: "ありがとうございます。", WeightCategory: "細部志向"},
		{Role: "user", Content: "はい"},
	}
	got, category := lastQuestionForScoring(history)
	if got != "ありがとうございます。" || category != "細部志向" {
		t.Fatalf("従来の挙動に落ちない: %q / %q", got, category)
	}
}

// 案内の目印が短いと、同じ言い回しを含む通常の質問まで案内として読み飛ばされ、
// その質問が採点対象から外れる（画面にも古い選択肢が残る）。
func TestGuidanceMarkerDoesNotSwallowNormalQuestions(t *testing.T) {
	normal := "あなたの考えに一番近いものを選んでください。当てはまらなければ内容も書いて送ってください。"
	if isValidationFeedbackMessage(normal) {
		t.Fatalf("通常の質問が案内として判定される: %q", normal)
	}
	if !isValidationFeedbackMessage(validationOtherNeedsTextMessage()) {
		t.Fatal("案内そのものが判定されない")
	}
}

// 案内が往復しても、採点は元の質問とその軸を使う。
// 以前は直近5件だけを取り直していたので、案内3往復で元の質問が窓から外れていた。
func TestScoringKeepsAxisAcrossRepeatedGuidance(t *testing.T) {
	question := buildChoiceFallback("新しいことに挑戦しましたか？", "成長志向")
	history := []models.ChatMessage{
		{Role: "assistant", Content: question, WeightCategory: "成長志向"},
	}
	for range 3 {
		history = append(history,
			models.ChatMessage{Role: "user", Content: "5"},
			models.ChatMessage{Role: "assistant", Content: validationOtherNeedsTextMessage()},
		)
	}
	if _, category := lastQuestionForScoring(history); category != "成長志向" {
		t.Fatalf("案内3往復で軸を失う: %q", category)
	}
}

// 「5:」のように区切りだけで本文が無いものも、内容なしの「その他」として扱う。
func TestEmptyOtherChoiceWithTrailingDelimiter(t *testing.T) {
	question := buildChoiceFallback("新しいことに挑戦しましたか？", "成長志向")
	for _, answer := range []string{"5", "5:", "5：", "5： ", "5。"} {
		if !isEmptyOtherChoiceAnswer(question, answer) {
			t.Fatalf("内容なしの「その他」として扱われない: %q", answer)
		}
	}
	// 本文があるものは通常どおり通す
	if isEmptyOtherChoiceAnswer(question, "5: 部活で新しい練習方法を考えました") {
		t.Fatal("本文のある「その他」まで案内に落ちている")
	}
}
