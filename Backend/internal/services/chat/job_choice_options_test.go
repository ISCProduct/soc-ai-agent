package chat

import (
	"testing"

	"Backend/internal/models"
)

// 本番データで実際に提示された小分類の質問文（session_1789406717449_nrukar, msg 66）。
const realClarificationQuestion = `「開発系エンジニア」とのことですが、以下のうちどれが一番近いですか？

1. ソフトウェアエンジニア（業務アプリ・システム開発）
2. Webエンジニア（Webサービス・フロント/バックエンド開発）
3. データエンジニア（データ基盤・分析基盤の構築）

番号で答えても、職種名で答えても構いません。`

// TestNormalizeNumericAnswer_UsesPresentedOptions は報告されたバグの回帰テスト。
//
// 「開発系エンジニア」と答えた学生に小分類の選択肢を提示し、「2」と返ってきた。
// 修正前は大分類マスタの2番目（営業）として登録され、以降の質問が
// 「営業として就職後の5〜10年を想像したとき…」になり、職種適性の採点も
// 営業基準で行われていた。
func TestNormalizeNumericAnswer_UsesPresentedOptions(t *testing.T) {
	// 大分類マスタ。2番目が営業であることが事故の原因だった。
	repo := &jobCategoryRepoMock{topCategories: []models.JobCategory{
		{Name: "エンジニア"}, {Name: "営業"}, {Name: "マーケティング"},
		{Name: "人事"}, {Name: "財務・経理"}, {Name: "経営コンサルタント"},
	}}
	v := &JobCategoryValidator{jobCategoryRepo: repo}

	got, err := v.normalizeNumericAnswer("2", realClarificationQuestion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "営業" {
		t.Fatalf("小分類の「2」を大分類の2番目(営業)として解釈している（報告されたバグ）")
	}
	if got != "Webエンジニア" {
		t.Fatalf("want Webエンジニア, got %s", got)
	}
}

func TestNormalizeNumericAnswer_PresentedOptionsForEachChoice(t *testing.T) {
	repo := &jobCategoryRepoMock{topCategories: []models.JobCategory{
		{Name: "エンジニア"}, {Name: "営業"}, {Name: "マーケティング"},
	}}
	v := &JobCategoryValidator{jobCategoryRepo: repo}

	for answer, want := range map[string]string{
		"1": "ソフトウェアエンジニア",
		"2": "Webエンジニア",
		"3": "データエンジニア",
	} {
		got, err := v.normalizeNumericAnswer(answer, realClarificationQuestion)
		if err != nil {
			t.Fatalf("answer=%s: %v", answer, err)
		}
		if got != want {
			t.Errorf("answer=%s: want %s, got %s", answer, want, got)
		}
	}
}

// 提示された範囲外の番号は大分類に当てない。当てると全く別の職種になる。
func TestNormalizeNumericAnswer_OutOfPresentedRangeKeepsOriginal(t *testing.T) {
	repo := &jobCategoryRepoMock{topCategories: []models.JobCategory{
		{Name: "エンジニア"}, {Name: "営業"}, {Name: "マーケティング"},
		{Name: "人事"}, {Name: "財務・経理"},
	}}
	v := &JobCategoryValidator{jobCategoryRepo: repo}

	got, err := v.normalizeNumericAnswer("5", realClarificationQuestion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "5" {
		t.Errorf("提示範囲外の番号をマスタに当てている: got %s", got)
	}
}

// 全角数字で答えても同じ選択肢を指すこと。
func TestNormalizeNumericAnswer_FullWidthDigit(t *testing.T) {
	v := &JobCategoryValidator{jobCategoryRepo: &jobCategoryRepoMock{}}
	got, err := v.normalizeNumericAnswer("２", realClarificationQuestion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Webエンジニア" {
		t.Errorf("want Webエンジニア, got %s", got)
	}
}

// LLM の出力は改行区切りとは限らない。1行に並べられても解釈できること。
// ここが抜けると、書式が変わっただけで元の不具合に戻る。
func TestExtractPresentedOptions_Inline(t *testing.T) {
	got := ExtractPresentedOptions("近いのはどれですか？ 1. ソフトウェアエンジニア 2. Webエンジニア 3. データエンジニア")
	want := []string{"ソフトウェアエンジニア", "Webエンジニア", "データエンジニア"}
	if len(got) != len(want) {
		t.Fatalf("件数 = %d (%v), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// 質問を提示しているのに選択肢を拾えなかった場合、大分類マスタへは当てない。
// 当てると書式が変わっただけで「2 → 営業」に戻る。
func TestNormalizeNumericAnswer_UnparsableQuestionDoesNotFallBackToMaster(t *testing.T) {
	repo := &jobCategoryRepoMock{topCategories: []models.JobCategory{
		{Name: "エンジニア"}, {Name: "営業"}, {Name: "マーケティング"},
	}}
	v := &JobCategoryValidator{jobCategoryRepo: repo}

	// 番号付きの選択肢が無い質問文
	got, err := v.normalizeNumericAnswer("2", "どんな仕事に興味がありますか？自由にお答えください。")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "営業" {
		t.Fatal("大分類マスタに当てている（書式変更で元の不具合に戻る経路）")
	}
	if got != "2" {
		t.Errorf("原文のまま AI 判定へ渡すこと: got %s", got)
	}
}

// 質問文が無いときだけ大分類に当てる（履歴が空の初回など）。
func TestNormalizeNumericAnswer_NoQuestionUsesMaster(t *testing.T) {
	repo := &jobCategoryRepoMock{topCategories: []models.JobCategory{
		{Name: "エンジニア"}, {Name: "営業"},
	}}
	v := &JobCategoryValidator{jobCategoryRepo: repo}

	got, err := v.normalizeNumericAnswer("2", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "営業" {
		t.Errorf("want 営業, got %s", got)
	}
}

func TestExtractPresentedOptions(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "本番の小分類質問",
			text: realClarificationQuestion,
			want: []string{"ソフトウェアエンジニア", "Webエンジニア", "データエンジニア"},
		},
		{
			// GenerateJobSelectionQuestion が作る形式。
			name: "大分類の選択肢",
			text: "どの職種に興味がありますか？以下から選んでください：\n\n1. エンジニア\n2. 営業\n3. まだ決めていない\n",
			want: []string{"エンジニア", "営業", "まだ決めていない"},
		},
		{
			name: "全角の番号と括弧",
			text: "近いのはどれですか？\n１．法人営業（BtoB）\n２．個人営業（BtoC）",
			want: []string{"法人営業", "個人営業"},
		},
		{
			// 選択肢が無い通常の質問。番号を拾ってはいけない。
			name: "選択肢のない質問",
			text: "学生生活でITツールや仕組みで作業を効率化した経験を教えてください。",
			want: nil,
		},
		{
			// 本文中にたまたま数字があるだけのもの。連番でなければ選択肢ではない。
			name: "連番でない数字は選択肢ではない",
			text: "3. の観点について、あなたの考えを教えてください。",
			want: nil,
		},
		{
			name: "1件だけは選択肢とみなさない",
			text: "次のとおりです。\n1. 概要",
			want: nil,
		},
		{
			// マーカーは複数あるが、連番として成立するのは1件だけ。
			// これを選択肢として採用すると、本文中の数字に番号を当ててしまう。
			name: "連番が1件しか成立しない場合も選択肢とみなさない",
			text: "1. 概要をまとめました。5. 詳細は別紙です。",
			want: nil,
		},
		{
			// 連番が途中で飛ぶ場合、飛んだ先は選択肢ではない。
			name: "番号が飛んだら以降は拾わない",
			text: "どれですか？\n1. エンジニア\n2. 営業\n7. その他",
			want: []string{"エンジニア", "営業"},
		},
		{
			name: "空文字",
			text: "",
			want: nil,
		},
		{
			// 番号だけで名前が無い。選択肢として使えないので採用しない。
			name: "名前の無い番号は選択肢とみなさない",
			text: "どれですか？\n1.\n2.\n",
			want: nil,
		},
		{
			// 一部だけ名前が欠けている場合も、番号と名前の対応が崩れるため採用しない。
			name: "一部の名前が欠けていたら採用しない",
			text: "どれですか？\n1. エンジニア\n2.\n3. 営業\n",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPresentedOptions(tt.text)
			if len(got) != len(tt.want) {
				t.Fatalf("件数 = %d (%v), want %d (%v)", len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestOptionForChoice(t *testing.T) {
	options := []string{"A", "B", "C"}
	if got := OptionForChoice(options, 2); got != "B" {
		t.Errorf("want B, got %s", got)
	}
	for _, n := range []int{0, -1, 4} {
		if got := OptionForChoice(options, n); got != "" {
			t.Errorf("choice=%d は空を返すこと: got %s", n, got)
		}
	}
	if got := OptionForChoice(nil, 1); got != "" {
		t.Errorf("選択肢なしは空を返すこと: got %s", got)
	}
}
