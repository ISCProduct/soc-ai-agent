package chat

import (
	"math"
	"strings"
	"unicode/utf8"
)

// 診断品質のための選択肢スコア調整。
//
// 選択肢は「軸上の位置」の粗い信号。理由なしの極端値をそのまま書くと、
// 会話根拠のない高マッチが量産される。理由があり矛盾しなければ本値、
// 理由なしは中立寄り、理由が選択と逆ならさらに中立へ戻す。

const (
	choiceOnlyDampening    = 0.55 // 理由なし: 50 からの距離を 55% に縮める
	contradictionDamping   = 0.25 // 矛盾時: 50 からの距離を 25% に縮める
	minReasonRunesEvidence = 8
)

var (
	highChoiceNegation = []string{"苦手", "やりたくない", "嫌", "向かない", "避け", "したくない", "興味がない", "向いていない"}
	lowChoiceAffirm    = []string{"得意", "好き", "やりたい", "向いている", "強み", "積極的", "大切にしたい"}
)

// AdjustChoiceAxisScore は選択肢の軸点を、理由の有無・矛盾で調整する。
// choiceScore は scoreChoice の出力（A=100…E=20）。
func AdjustChoiceAxisScore(choiceScore int, reason string) (adjusted int, flags []string) {
	reason = strings.TrimSpace(reason)
	hasReason := utf8.RuneCountInString(reason) >= minReasonRunesEvidence
	contradicts := hasReason && choiceReasonContradicts(choiceScore, reason)

	switch {
	case !hasReason:
		adjusted = dampenTowardNeutral(choiceScore, choiceOnlyDampening)
		flags = append(flags, "choice_only_evidence")
	case contradicts:
		adjusted = dampenTowardNeutral(choiceScore, contradictionDamping)
		flags = append(flags, "choice_reason_contradiction")
	default:
		adjusted = choiceScore
		flags = append(flags, "choice_with_supporting_reason")
	}
	return adjusted, flags
}

func choiceReasonContradicts(choiceScore int, reason string) bool {
	// 高選択（>=80）なのに否定語、低選択（<=40）なのに肯定語
	if choiceScore >= 80 {
		for _, w := range highChoiceNegation {
			if strings.Contains(reason, w) {
				return true
			}
		}
	}
	if choiceScore <= 40 {
		for _, w := range lowChoiceAffirm {
			if strings.Contains(reason, w) {
				return true
			}
		}
	}
	return false
}

func dampenTowardNeutral(score int, factor float64) int {
	// 50 + (score-50)*factor
	v := 50.0 + float64(score-50)*factor
	return int(math.Round(math.Max(0, math.Min(100, v))))
}

// 会話履歴上のユーザー発言の根拠分類（品質ジョブ用）。
const (
	EvidenceEmpty               = "empty"
	EvidenceChoiceOnly          = "choice_only"
	EvidenceChoiceWithReason    = "choice_with_reason"
	EvidenceChoiceContradiction = "choice_contradiction"
	EvidenceThinFreeText        = "thin_free_text"
	EvidenceFreeText            = "free_text"
)

// ClassifyOutgoingAnswerEvidence は保存済みユーザー発言の根拠の厚さを分類する。
func ClassifyOutgoingAnswerEvidence(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return EvidenceEmpty
	}
	if letter, reason, ok := SplitChoiceAndReason(content); ok {
		_, flags := AdjustChoiceAxisScore(letterRawScore(letter), reason)
		for _, f := range flags {
			if f == "choice_reason_contradiction" {
				return EvidenceChoiceContradiction
			}
			if f == "choice_only_evidence" {
				return EvidenceChoiceOnly
			}
		}
		return EvidenceChoiceWithReason
	}
	if isChoiceToken(strings.ToUpper(content)) {
		return EvidenceChoiceOnly
	}
	if utf8.RuneCountInString(content) < minReasonRunesEvidence {
		return EvidenceThinFreeText
	}
	return EvidenceFreeText
}

func letterRawScore(letter string) int {
	switch strings.ToUpper(strings.TrimSpace(letter)) {
	case "A", "1":
		return 100
	case "B", "2":
		return 80
	case "C", "3":
		return 60
	case "D", "4":
		return 40
	case "E", "5":
		return 20
	default:
		return 60
	}
}
