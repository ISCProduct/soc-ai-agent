package chat

import (
	"regexp"
	"strings"
	"unicode"
)

// ParsedChoice は質問文から抽出した選択肢。
type ParsedChoice struct {
	Value string // A-E または 1-5
	Text  string
}

var (
	choiceLetterLine = regexp.MustCompile(`^([A-Ea-e])[)）：、.．]\s*(.+)$`)
	choiceNumberLine = regexp.MustCompile(`^([1-5])[)）．.]\s*(.+)$`)
)

// ParseChoiceOptions は質問文から A)/1. 形式の選択肢を抽出する。
func ParseChoiceOptions(question string) []ParsedChoice {
	var out []ParsedChoice
	for _, line := range strings.Split(question, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if m := choiceLetterLine.FindStringSubmatch(trimmed); m != nil {
			out = append(out, ParsedChoice{Value: strings.ToUpper(m[1]), Text: strings.TrimSpace(m[2])})
			continue
		}
		if m := choiceNumberLine.FindStringSubmatch(trimmed); m != nil {
			out = append(out, ParsedChoice{Value: m[1], Text: strings.TrimSpace(m[2])})
		}
	}
	return out
}

func isOtherChoiceText(text string) bool {
	return strings.Contains(text, "その他")
}

func normalizeChoiceKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsSpace(r) {
			continue
		}
		switch r {
		case '、', '。', '・', '（', '）', '(', ')', ':', '：', '.', '．', '/', '／':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func choiceTextsMatch(answer, option string) bool {
	a := normalizeChoiceKey(answer)
	o := normalizeChoiceKey(option)
	if a == "" || o == "" {
		return false
	}
	if a == o {
		return true
	}
	// 短すぎる部分一致は誤爆しやすいので 4 文字以上に限定
	if len([]rune(a)) >= 4 && len([]rune(o)) >= 4 {
		return strings.Contains(a, o) || strings.Contains(o, a)
	}
	return false
}

// ChoiceResolution はユーザー入力を選択肢記号または自由記述へ正規化した結果。
type ChoiceResolution struct {
	Letter     string // 選択肢記号（IsChoice=true のとき）
	IsChoice   bool   // processChoiceAnswer 経路
	IsFreeText bool   // 文章採点経路（その他・非選択肢質問含む）
	Text       string // 採点・保存に使う本文（自由記述時）
	Reason     string // 選択肢に付随する任意の理由テキスト
}

// choiceWithReasonRe は FE が送る "A: 理由" / "1：理由" 形式。
var choiceWithReasonRe = regexp.MustCompile(`(?s)^([A-Ea-e1-5])\s*[:：]\s*(.+)$`)

// SplitChoiceAndReason は "A: 理由" 形式を letter と理由に分ける。
// コロン形式でなければ letter="", reason=原文。
func SplitChoiceAndReason(answer string) (letter, reason string, ok bool) {
	answer = strings.TrimSpace(answer)
	m := choiceWithReasonRe.FindStringSubmatch(answer)
	if m == nil {
		return "", answer, false
	}
	letter = strings.ToUpper(m[1])
	if letter >= "1" && letter <= "5" {
		// 数字はそのまま
		letter = m[1]
	}
	reason = strings.TrimSpace(m[2])
	return letter, reason, true
}

// ResolveChoiceAnswer は選択肢質問に対する自由入力を記号へ寄せる。
// マッチしない自由記述・「その他」は IsFreeText=true（scoreChoice default 60 を避ける）。
// "A: 理由" 形式は IsChoice=true かつ Reason に理由を載せる。
func ResolveChoiceAnswer(question, answer string) ChoiceResolution {
	answer = strings.TrimSpace(answer)
	options := ParseChoiceOptions(question)
	if len(options) == 0 {
		return ChoiceResolution{IsFreeText: true, Text: answer}
	}

	if letter, reason, ok := SplitChoiceAndReason(answer); ok {
		for _, opt := range options {
			if !strings.EqualFold(opt.Value, letter) {
				continue
			}
			if isOtherChoiceText(opt.Text) {
				// 「その他」は記号採点に落とさず自由記述として扱う
				return ChoiceResolution{IsFreeText: true, Text: reason}
			}
			return ChoiceResolution{Letter: opt.Value, IsChoice: true, Text: opt.Value, Reason: reason}
		}
		return ChoiceResolution{Letter: letter, IsChoice: true, Text: letter, Reason: reason}
	}

	upper := strings.ToUpper(answer)
	if isChoiceToken(upper) {
		for _, opt := range options {
			if strings.EqualFold(opt.Value, upper) {
				return ChoiceResolution{Letter: opt.Value, IsChoice: true, Text: opt.Value}
			}
		}
		return ChoiceResolution{Letter: upper, IsChoice: true, Text: upper}
	}

	for _, opt := range options {
		if isOtherChoiceText(opt.Text) {
			continue
		}
		if choiceTextsMatch(answer, opt.Text) {
			return ChoiceResolution{Letter: opt.Value, IsChoice: true, Text: opt.Value}
		}
	}

	return ChoiceResolution{IsFreeText: true, Text: answer}
}

func isChoiceToken(answer string) bool {
	switch answer {
	case "A", "B", "C", "D", "E", "1", "2", "3", "4", "5":
		return true
	default:
		return false
	}
}
