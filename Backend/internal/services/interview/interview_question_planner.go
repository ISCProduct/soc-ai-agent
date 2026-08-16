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

	maxFollowUpDepth  = 2
	minDeepeningRunes = 40
)

// strongSignalPattern はそれだけで回答の具体性を裏付ける強いシグナル
// （数値・固有の取り組み内容）。
var strongSignalPattern = regexp.MustCompile(`\d|[％%]|万円|プロジェクト|実装|リリース`)

// weakSignalPattern は「開発に興味がある」「業務改善に貢献したい」のように
// 決意表明・抽象的な志望動機でも使われがちな動詞的シグナル。
// 単独では具体性の裏付けにならず、複数含まれる場合のみ具体的とみなす（#882）。
var weakSignalPattern = regexp.MustCompile(`担当|開発|改善`)

// numberedCountPattern は数字（半角・全角・漢数字）直後の「件」「人」のみを
// 数量表現として検出する。「案件」「個人」等への部分一致誤検知を避けるため、
// 単独の「件」「人」はシグナルに含めない（#881/#882）。
var numberedCountPattern = regexp.MustCompile(`[0-9０-９一二三四五六七八九十百千万]+\s*(件|人)`)

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

// isAbstractOnlyAnswer は回答が具体性シグナル（数値・固有の取り組み内容）を
// 一つも含まないかを判定する。
// #881: 従来は限定的な8キーワード（頑張・チームワーク等）に一致した場合のみ
// 抽象回答とみなしていたため、これらの語を含まない曖昧な回答（例:
// 「御社のビジョンに共感し貢献したい」等の定型文）が深掘りされずに素通りしていた。
// 特定キーワードへの一致に頼らず、具体性シグナルの欠如そのものを判定基準にする。
//
// #882: ただし「担当」「開発」「改善」等は「業務改善に貢献したい」のような
// 抽象的な決意表明にも使われる弱いシグナルのため、これらの単独一致だけでは
// 具体的と判定しない（強いシグナルとの併用、または弱いシグナルが複数ある
// 場合のみ具体的とみなす）。数量表現は「案件」「個人」等との部分一致を
// 避けるため、数字直後の「件」「人」のみを検出する。
func isAbstractOnlyAnswer(answer string) bool {
	if strongSignalPattern.MatchString(answer) {
		return false
	}
	if numberedCountPattern.MatchString(answer) {
		return false
	}
	if len(weakSignalPattern.FindAllString(answer, -1)) >= 2 {
		return false
	}
	return true
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
			"required_total":    requiredTotal,
			"required_asked":    requiredAsked,
			"required_answered": requiredAnswered,
			"required_coverage": coverageRate,
			"custom_total":      customTotal,
			"custom_asked":      customAsked,
		},
		"deepening_count":            deepeningCount,
		"unasked_required_questions": unaskedRequired,
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
