package interview

import (
	"fmt"
	"strings"

	"Backend/internal/models"
)

// evaluationIntentNote は「評価したい観点」の扱いを面接官へ伝える指示（#1092）。
//
// 観点は評価の基準そのものなので、応募者へ読み上げると質問の狙いが筒抜けになる。
// 観点自体は質問と同じ非信頼テキストの囲みの中に入れ（囲みの外に置くと迂回路になる）、
// この指示だけを囲みの外に置く。
const evaluationIntentNote = "「評価したい観点」は、その質問で確かめたいことを示す面接官向けの参考情報です。応募者には伝えず、回答の深掘りの方向づけにだけ使ってください。"

func evaluationIntentOf(q models.InterviewCompanyQuestion) string {
	if q.EvaluationIntent == nil {
		return ""
	}
	return strings.TrimSpace(*q.EvaluationIntent)
}

// evaluationIntentFor は質問キューの1問（directive）の元になったカスタム質問の観点を返す。
// 面接の途中で観点が書き換えられても、最新の内容を使う。
func evaluationIntentFor(qs []models.InterviewCompanyQuestion, id *uint) string {
	if id == nil {
		return ""
	}
	for _, q := range qs {
		if q.ID == *id {
			return evaluationIntentOf(q)
		}
	}
	return ""
}

func anyEvaluationIntent(qs []models.InterviewCompanyQuestion) bool {
	for _, q := range qs {
		if evaluationIntentOf(q) != "" {
			return true
		}
	}
	return false
}

// customQuestionLine は【必須質問】【推奨質問】の1行。観点は質問と同じ行の直下に入れる。
func customQuestionLine(q models.InterviewCompanyQuestion) string {
	line := fmt.Sprintf("- [%s] %s", q.Category, q.QuestionText)
	if intent := evaluationIntentOf(q); intent != "" {
		line += fmt.Sprintf("\n  評価したい観点: %s", intent)
	}
	return line
}
