package interview

import (
	"Backend/internal/models"
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	questionSourceCustom   = "custom"
	questionSourceTopic    = "topic"
	questionSourceFollowUp = "follow_up"

	questionStatusPending  = "pending"
	questionStatusAsked    = "asked"
	questionStatusAnswered = "answered"

	maxFollowUpDepth   = 2
	minDeepeningRunes  = 40
)

var abstractAnswerKeywords = []string{
	"頑張", "チームワーク", "コミュニケーション", "協力", "努力", "学び", "真面目", "前向き",
}

var specificSignalPattern = regexp.MustCompile(`\d|[％%]|万円|件|人|プロジェクト|担当|実装|開発|リリース|改善`)

type questionDirective struct {
	Text        string
	Source      string
	Category    string
	IsDeepening bool
}

// BuildQuestionQueue は必須カスタム質問→標準トピック→推奨カスタム質問の順でキューを構築する。
func BuildQuestionQueue(sessionID uint, customQuestions []models.InterviewCompanyQuestion) []models.InterviewQuestionState {
	states := make([]models.InterviewQuestionState, 0)
	order := 0

	appendCustom := func(required bool) {
		for _, q := range customQuestions {
			if q.IsRequired != required {
				continue
			}
			qid := q.ID
			states = append(states, models.InterviewQuestionState{
				SessionID:         sessionID,
				CompanyQuestionID: &qid,
				Source:            questionSourceCustom,
				Category:          q.Category,
				QuestionText:      q.QuestionText,
				Status:            questionStatusPending,
				SortOrder:         order,
				IsRequired:        q.IsRequired,
			})
			order++
		}
	}

	appendCustom(true)
	for _, topic := range interviewTopics {
		states = append(states, models.InterviewQuestionState{
			SessionID:    sessionID,
			Source:       questionSourceTopic,
			Category:     topic,
			QuestionText: topic,
			Status:       questionStatusPending,
			SortOrder:    order,
		})
		order++
	}
	appendCustom(false)
	return states
}

// NeedsDeepening は回答が抽象的すぎて追質問が必要かを判定する。
func NeedsDeepening(answer string, depth int) bool {
	if depth >= maxFollowUpDepth {
		return false
	}
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" || trimmed == "（聞き取れませんでした）" {
		return true
	}
	if utf8.RuneCountInString(trimmed) < minDeepeningRunes {
		return true
	}
	return isAbstractOnlyAnswer(trimmed)
}

func isAbstractOnlyAnswer(answer string) bool {
	hasAbstract := false
	for _, kw := range abstractAnswerKeywords {
		if strings.Contains(answer, kw) {
			hasAbstract = true
			break
		}
	}
	if !hasAbstract {
		return false
	}
	return !specificSignalPattern.MatchString(answer)
}

// BuildFollowUpQuestionText は深掘り追質問のテンプレート文を返す。
func BuildFollowUpQuestionText(originalQuestion, userAnswer string) string {
	snippet := strings.TrimSpace(userAnswer)
	if utf8.RuneCountInString(snippet) > 40 {
		snippet = string([]rune(snippet)[:40]) + "…"
	}
	if snippet == "" {
		snippet = "先ほどのお話"
	}
	if strings.TrimSpace(originalQuestion) != "" {
		return "「" + snippet + "」について、もう少し具体的にお聞きします。" + originalQuestion + " の中で、あなた自身の役割や取った行動を教えてください。"
	}
	return "「" + snippet + "」について、具体的なエピソードやあなたの役割を教えてください。"
}

func selectNextPending(states []models.InterviewQuestionState) *models.InterviewQuestionState {
	for i := range states {
		if states[i].Status == questionStatusPending {
			return &states[i]
		}
	}
	return nil
}

func buildQuestionCoverage(states []models.InterviewQuestionState) map[string]any {
	requiredTotal := 0
	requiredAsked := 0
	requiredAnswered := 0
	customTotal := 0
	customAsked := 0
	deepeningCount := 0
	unaskedRequired := make([]string, 0)

	for _, st := range states {
		if st.Source == questionSourceFollowUp {
			deepeningCount++
			continue
		}
		if st.Source != questionSourceCustom {
			continue
		}
		customTotal++
		if st.Status == questionStatusAsked || st.Status == questionStatusAnswered {
			customAsked++
		}
		if !st.IsRequired {
			continue
		}
		requiredTotal++
		switch st.Status {
		case questionStatusAsked:
			requiredAsked++
		case questionStatusAnswered:
			requiredAsked++
			requiredAnswered++
		case questionStatusPending:
			unaskedRequired = append(unaskedRequired, st.QuestionText)
		}
	}

	coverageRate := 0.0
	if requiredTotal > 0 {
		coverageRate = float64(requiredAsked) / float64(requiredTotal)
	}

	return map[string]any{
		"custom_questions_coverage": map[string]any{
			"required_total":     requiredTotal,
			"required_asked":     requiredAsked,
			"required_answered":  requiredAnswered,
			"required_coverage":  coverageRate,
			"custom_total":       customTotal,
			"custom_asked":       customAsked,
		},
		"deepening_count":             deepeningCount,
		"unasked_required_questions":  unaskedRequired,
	}
}

func mergeTeacherReportWithCoverage(teacherJSON string, coverage map[string]any) (string, error) {
	merged := map[string]any{}
	if strings.TrimSpace(teacherJSON) != "" && teacherJSON != "{}" {
		if err := json.Unmarshal([]byte(teacherJSON), &merged); err != nil {
			return teacherJSON, err
		}
	}
	for k, v := range coverage {
		merged[k] = v
	}
	out, err := json.Marshal(merged)
	if err != nil {
		return teacherJSON, err
	}
	return string(out), nil
}
