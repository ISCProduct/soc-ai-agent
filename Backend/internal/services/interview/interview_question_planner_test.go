package interview

import (
	"Backend/internal/models"
	"strings"
	"testing"
)

func TestBuildQuestionQueueOrdersRequiredBeforeTopics(t *testing.T) {
	custom := []models.InterviewCompanyQuestion{
		{ID: 1, Category: "技術", QuestionText: "推奨質問", IsRequired: false, Priority: 2},
		{ID: 2, Category: "志望動機", QuestionText: "必須質問", IsRequired: true, Priority: 1},
	}
	queue := BuildQuestionQueue(10, custom)
	if len(queue) != len(interviewTopics)+2 {
		t.Fatalf("queue length = %d, want %d", len(queue), len(interviewTopics)+2)
	}
	if !queue[0].IsRequired || queue[0].QuestionText != "必須質問" {
		t.Fatalf("first item = %+v, want required custom question", queue[0])
	}
	if queue[len(queue)-1].QuestionText != "推奨質問" {
		t.Fatalf("last item = %+v, want recommended custom question", queue[len(queue)-1])
	}
}

func TestNeedsDeepening(t *testing.T) {
	tests := []struct {
		name   string
		answer string
		depth  int
		want   bool
	}{
		{name: "短い回答", answer: "頑張りました。", depth: 0, want: true},
		{name: "具体性あり", answer: "3名チームでAPIを2週間以内にリリースし、レスポンスを40%改善しました。自分は設計とレビューを担当しました。", depth: 0, want: false},
		{name: "深掘り上限", answer: "頑張りました。", depth: 2, want: false},
		// #881: 特定8キーワードに一致しない曖昧な定型回答（40文字以上）も深掘り対象にする
		{name: "定型的な志望動機（キーワード非依存）", answer: "はい、御社のビジョンにとても共感しておりまして、これまでの経験を活かして貢献したいと考えております。", depth: 0, want: true},
		{name: "定型的な協調性アピール（キーワード非依存）", answer: "周囲とうまく連携しながら、状況に応じて柔軟に対応することを心がけて業務に取り組んできました。", depth: 0, want: true},
		// #881: 「案件」の「件」が specificSignalPattern に部分一致して誤って具体的と判定されないこと
		{name: "「案件」の部分一致誤検知が起きない", answer: "色々な案件に携わってきましたが、その中でも特に印象に残っているものがいくつかあり、そこから多くのことを学びました。", depth: 0, want: true},
		{name: "具体的なプロジェクト名・数値あり（深掘り不要）", answer: "前職ではECサイトのリニューアルプロジェクトを担当し、決済APIの実装とパフォーマンス改善を行い、ページ表示速度を約40%改善しました。", depth: 0, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsDeepening(tt.answer, tt.depth); got != tt.want {
				t.Fatalf("NeedsDeepening() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildQuestionCoverage(t *testing.T) {
	qid := uint(99)
	states := []models.InterviewQuestionState{
		{Source: questionSourceCustom, QuestionText: "必須1", IsRequired: true, Status: questionStatusAnswered},
		{Source: questionSourceCustom, QuestionText: "必須2", IsRequired: true, Status: questionStatusPending, CompanyQuestionID: &qid},
		{Source: questionSourceFollowUp, QuestionText: "深掘り", Status: questionStatusAnswered},
	}
	coverage := buildQuestionCoverage(states)
	required := coverage["custom_questions_coverage"].(map[string]any)
	if required["required_total"].(int) != 2 {
		t.Fatalf("required_total = %v", required["required_total"])
	}
	if required["required_asked"].(int) != 1 {
		t.Fatalf("required_asked = %v", required["required_asked"])
	}
	if coverage["deepening_count"].(int) != 1 {
		t.Fatalf("deepening_count = %v", coverage["deepening_count"])
	}
	unasked := coverage["unasked_required_questions"].([]string)
	if len(unasked) != 1 || unasked[0] != "必須2" {
		t.Fatalf("unasked = %#v", unasked)
	}
}

func TestBuildInterviewSystemPromptWithDirective(t *testing.T) {
	prompt := buildInterviewSystemPrompt(
		"テスト株式会社", "", "エンジニア", "", "general",
		nil, nil, 0, 0, 1, 5, 0, 180,
		&questionDirective{Text: "具体的な失敗経験を教えてください。", Source: questionSourceCustom, Category: "強み・弱み", IsDeepening: true},
	)
	if !containsAll(prompt, []string{"【今回の質問】", "深掘り質問", "具体的な失敗経験を教えてください。"}) {
		t.Fatalf("prompt missing directive section: %s", prompt)
	}
}

func containsAll(s string, parts []string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
