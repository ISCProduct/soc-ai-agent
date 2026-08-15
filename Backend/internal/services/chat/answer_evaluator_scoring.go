package chat

import (
	"Backend/internal/services/shared"
	"math"
	"regexp"
	"strings"
)

func (e *AnswerEvaluator) InferCategory(question string) string {
	return e.categorizeQuestion(question)
}

func (e *AnswerEvaluator) categorizeQuestion(question string) string {
	q := strings.ToLower(question)
	if shared.ContainsAny(q, []string{"興味", "きっかけ", "理由", "魅力", "なぜ"}) {
		return "motivation"
	}
	if shared.ContainsAny(q, []string{"作った", "開発", "実装", "制作", "プロジェクト", "成果物", "github"}) {
		return "experience"
	}
	if shared.ContainsAny(q, []string{"意見", "食い違い", "合意", "調整", "衝突", "まとめ", "折衷", "合意形成"}) {
		return "collaboration"
	}
	if shared.ContainsAny(q, []string{"itに詳しくない", "職員", "説明", "理解確認", "伝え方", "使い方", "現場"}) {
		return "communication_non_it"
	}
	if shared.ContainsAny(q, []string{"ui", "ux", "使いやすさ", "試して", "ユーザ", "改善", "反応", "導線"}) {
		return "ui_ux"
	}
	return "generic"
}

func rubricForCategory(category string) string {
	switch category {
	case "collaboration":
		return "collaboration_rubric"
	case "communication_non_it":
		return "communication_non_it_rubric"
	case "ui_ux":
		return "ui_ux_rubric"
	default:
		return "generic_text_rubric"
	}
}

type signalSet struct {
	hasConcreteExample   bool
	hasAction            bool
	hasResult            bool
	hasReason            bool
	hasNumbersOrTime     bool
	hasEmotion           bool
	hasCollaborationTerm bool
	hasNonITTerm         bool
	hasUxTerm            bool
	contradiction        bool
}

// hasEngagement は関与シグナルが1つでもあるか（#561: too_generic 判定に使用）
func (s signalSet) hasEngagement() bool {
	return s.hasConcreteExample || s.hasAction || s.hasResult || s.hasReason ||
		s.hasNumbersOrTime || s.hasEmotion || s.hasCollaborationTerm || s.hasUxTerm
}

func extractSignals(answer string, category string) signalSet {
	lower := strings.ToLower(answer)
	_ = category
	return signalSet{
		hasConcreteExample: shared.ContainsAny(lower, []string{"例えば", "たとえば", "具体的", "実際に", "経験", "した時", "したとき"}),
		// 凝縮表現（役割分担・リリース・出した等）も行動シグナルとして扱う (#561)
		hasAction: shared.ContainsAny(lower, []string{
			"取り組", "実施", "作成", "作った", "実装", "改善", "対応", "開発", "設計", "検証",
			"分担", "役割", "リリース", "出した", "進め", "仕上げ", "完了", "提出", "納品", "デプロイ",
			"まとめた", "進めた", "対応した", "リリースした",
		}),
		hasResult: shared.ContainsAny(lower, []string{
			"結果", "成果", "達成", "改善された", "向上", "成功", "失敗",
			"リリース", "出した", "出荷", "納品",
		}),
		hasReason: shared.ContainsAny(lower, []string{"理由", "なぜ", "ので", "ため", "から", "だから"}),
		hasNumbersOrTime: regexp.MustCompile(`[0-9]`).MatchString(lower) || shared.ContainsAny(lower, []string{
			"ヶ月", "年", "週間", "日間", "日で", "%", "人", "回",
		}),
		hasEmotion: shared.ContainsAny(lower, []string{
			"やりがい", "面白", "楽しい", "嬉しかっ", "充実", "役立", "人の役", "感動", "好き",
		}),
		hasCollaborationTerm: shared.ContainsAny(lower, []string{
			"合意", "調整", "衝突", "折衷", "意見", "まとめ", "チーム", "分担",
		}),
		hasNonITTerm: shared.ContainsAny(lower, []string{
			"itに詳しくない", "非エンジニア", "職員", "現場", "利用者",
		}),
		hasUxTerm: shared.ContainsAny(lower, []string{
			"ui", "ux", "ユーザ", "導線", "使いやす", "反応", "テスト",
		}),
		contradiction: false,
	}
}

func scoreDimensions(rubric string, signals signalSet, answer string) map[string]int {
	length := len([]rune(strings.TrimSpace(answer)))
	scores := map[string]int{
		"relevance":   1,
		"specificity": 0,
		"reasoning":   0,
		"credibility": 1,
	}

	// 理想的・定型的な回答に対するペナルティ
	isTooPerfect := !signals.hasReason && !signals.hasNumbersOrTime && length > 50 && signals.hasAction && signals.hasResult

	if rubric == "collaboration_rubric" && !signals.hasCollaborationTerm {
		scores["relevance"] = 1
	} else if rubric == "communication_non_it_rubric" && !signals.hasNonITTerm {
		scores["relevance"] = 1
	} else if rubric == "ui_ux_rubric" && !signals.hasUxTerm {
		scores["relevance"] = 1
	}

	if signals.hasConcreteExample && signals.hasAction {
		scores["relevance"] = 3
	} else if signals.hasAction || signals.hasResult || signals.hasConcreteExample {
		scores["relevance"] = 2
	}

	if signals.hasConcreteExample && signals.hasNumbersOrTime {
		scores["specificity"] = 3
	} else if signals.hasConcreteExample {
		scores["specificity"] = 2
	} else if signals.hasNumbersOrTime || signals.hasAction || signals.hasResult {
		scores["specificity"] = 1
	}

	if signals.hasReason && (signals.hasAction || signals.hasResult) {
		scores["reasoning"] = 3
	} else if signals.hasReason || signals.hasEmotion {
		scores["reasoning"] = 2
	} else if signals.hasConcreteExample {
		scores["reasoning"] = 1
	}

	if signals.contradiction {
		scores["credibility"] = 0 // 矛盾あり: 最低評価
	} else if isTooPerfect {
		scores["credibility"] = 1 // 理由・数値のない定型回答: 信頼度を落とす
	} else if signals.hasNumbersOrTime {
		scores["credibility"] = 3
	} else if signals.hasConcreteExample || (signals.hasEmotion && signals.hasReason) {
		scores["credibility"] = 2
	}

	return scores
}

func scoreFromDimensions(rubric string, dimensionScores map[string]int) int {
	weights := map[string]float64{
		"relevance":   0.35,
		"specificity": 0.30,
		"reasoning":   0.20,
		"credibility": 0.15,
	}

	switch rubric {
	case "collaboration_rubric":
		weights["specificity"] = 0.35
		weights["reasoning"] = 0.30
	case "communication_non_it_rubric":
		weights["relevance"] = 0.35
		weights["reasoning"] = 0.30
	case "ui_ux_rubric":
		weights["specificity"] = 0.35
	}

	sum := 0.0
	maxSum := 0.0
	for key, weight := range weights {
		sum += weight * float64(dimensionScores[key])
		maxSum += weight * 3.0
	}
	if maxSum == 0 {
		return 0
	}
	score := (sum / maxSum) * 100.0
	return int(math.Round(score))
}

func applyPenaltiesAndBoosts(score int, signals signalSet, answerLen int) (int, []string, []string) {
	penalties := []string{}
	boosts := []string{}
	finalScore := score

	// #561: 関与シグナルが1つでもあれば too_generic を適用しない
	if !signals.hasEngagement() {
		finalScore -= 5
		penalties = append(penalties, "too_generic")
	}
	if signals.contradiction {
		finalScore -= 20
		penalties = append(penalties, "contradiction")
	}
	if signals.hasNumbersOrTime {
		finalScore += 5
		boosts = append(boosts, "evidence")
	}
	// 短文でも行動・理由・感情などが凝縮されている場合はブースト (#561)
	if answerLen > 0 && answerLen <= 30 && signals.hasEngagement() {
		finalScore += 8
		boosts = append(boosts, "condensed_engagement")
	}

	if finalScore < 0 {
		finalScore = 0
	}
	if finalScore > 100 {
		finalScore = 100
	}

	return finalScore, penalties, boosts
}

func (e *AnswerEvaluator) scoreChoice(answer string, meta *questionMeta) int {
	choice := strings.ToUpper(strings.TrimSpace(answer))
	direction := "neutral"
	if meta != nil && meta.ChoiceSemantics != "" {
		direction = meta.ChoiceSemantics
	}

	switch direction {
	case "higher_is_better":
		return mapChoice(choice, []int{100, 80, 60, 40, 20})
	case "lower_is_better":
		return mapChoice(choice, []int{20, 40, 60, 80, 100})
	default:
		return mapChoice(choice, []int{100, 80, 60, 40, 20})
	}
}

func mapChoice(choice string, values []int) int {
	switch choice {
	case "A", "1":
		return values[0]
	case "B", "2":
		return values[1]
	case "C", "3":
		return values[2]
	case "D", "4":
		return values[3]
	case "E", "5":
		return values[4]
	default:
		return 60
	}
}
