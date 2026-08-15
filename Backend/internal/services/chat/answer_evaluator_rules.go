package chat

import (
	"Backend/internal/models"
	"fmt"
	"regexp"
	"strings"
)

func (e *AnswerEvaluator) evaluateRule(rule models.ScoreRule, answer string, answerLength int) bool {
	answerLower := strings.ToLower(answer)

	switch rule.Condition {
	case "contains_any":
		// いずれかのキーワードを含む
		for _, keyword := range rule.Keywords {
			if strings.Contains(answerLower, strings.ToLower(keyword)) {
				return true
			}
		}
		return false

	case "contains_all":
		// すべてのキーワードを含む
		for _, keyword := range rule.Keywords {
			if !strings.Contains(answerLower, strings.ToLower(keyword)) {
				return false
			}
		}
		return true

	case "length_gt":
		// 文字数が指定値より大きい
		if len(rule.Keywords) > 0 {
			threshold := 0
			fmt.Sscanf(rule.Keywords[0], "%d", &threshold)
			return answerLength > threshold
		}
		return false

	case "length_lt":
		// 文字数が指定値より小さい
		if len(rule.Keywords) > 0 {
			threshold := 0
			fmt.Sscanf(rule.Keywords[0], "%d", &threshold)
			return answerLength < threshold
		}
		return false

	case "regex":
		// 正規表現マッチ
		if len(rule.Keywords) > 0 {
			pattern := rule.Keywords[0]
			matched, err := regexp.MatchString(pattern, answer)
			if err != nil {
				return false
			}
			return matched
		}
		return false

	case "has_example":
		// 具体例を含んでいるか（「例えば」「たとえば」「〜した時」など）
		examplePatterns := []string{
			"例えば", "たとえば", "具体的には", "実際に", "した時", "したとき",
			"経験", "〜で", "〜では", "ことがあ",
		}
		for _, pattern := range examplePatterns {
			if strings.Contains(answerLower, pattern) {
				return true
			}
		}
		return false

	default:
		return false
	}
}

// shouldTriggerFollowUp 追加質問が必要か判定
func (e *AnswerEvaluator) shouldTriggerFollowUp(rule models.FollowUpRule, result *EvaluationResult) bool {
	switch rule.Trigger {
	case "low_confidence":
		return result.Confidence == "low"

	case "high_score":
		return result.Score >= 5

	case "no_keywords":
		return len(result.MatchedKeywords) == 0

	case "negative_keyword":
		return result.FollowUpTrigger == "negative_keyword"

	default:
		return false
	}
}

// GetConfidenceLevel スコアから信頼度レベルを取得
func (e *AnswerEvaluator) GetConfidenceLevel(score int, keywordCount int) string {
	if score <= 0 || keywordCount == 0 {
		return "low"
	}
	if score >= 5 && keywordCount >= 2 {
		return "high"
	}
	return "medium"
}
