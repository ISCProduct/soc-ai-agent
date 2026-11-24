package prompts

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// SOCAIAGENT-661: 出力スキーマは Go の構造体を正とし、プロンプトの出力形式はそこから作る。

// 構造体から作るように変えた時点の本文。本文が変わると版の記録がずれるので、
// この変更で本文が1文字も変わっていないことを固定する。スキーマを意図して変えたときは、
// 版（declaredPromptVersions）を上げたうえでここも書き換える。
const answerQualityPromptAtV2 = `あなたは就職面接の回答品質を評価する専門家です。
以下の観点でJSON形式のみで評価してください。

## 評価観点
- score: 総合スコア（0〜100の整数）
- confidence: 評価の信頼度 "high" / "medium" / "low"
- specificity: 具体性（0〜3）- 具体的な経験・数値・エピソードの有無
- authenticity: 本音度（0〜3）- 定型文でなく個人の実感が感じられるか
- consistency: 一貫性（0〜3）- 論理的で矛盾がないか
- explanation: 評価理由（日本語、60文字以内）

## 判定基準
- 短くても「なぜ」「どう行動したか」が含まれれば高評価にする
- 「やりがいを感じました」のような感情表現も本音度が高ければ confidence: "high" にする
- キーワード数より内容の質と深さを重視する
- 長文でも内容が薄い場合は低評価にする

## confidence の目安
- "high": score >= 70
- "medium": score >= 40
- "low": score < 40

## 重要な制約
- 必ずJSONオブジェクトのみを返す。説明文や前置きは一切不要。`

const answerValidationPromptAtV2 = SystemRoleValidator + `

## 重要な制約
- 必ずJSON形式のみで応答してください
- 他の説明文やコメントは一切含めないでください

## 出力形式（厳守）
{"valid": true, "reason": "...", "confidence": 0.0} または {"valid": false, "reason": "...", "confidence": 0.0}
(キーは必須。confidence は 0.0〜1.0 の範囲の数値を返してください)`

func TestOutputSchema_構造体から作った本文は以前と同じ(t *testing.T) {
	if AnswerQualitySystemPrompt != answerQualityPromptAtV2 {
		t.Errorf("AnswerQualitySystemPrompt が変わった:\n%s", AnswerQualitySystemPrompt)
	}
	if AnswerValidationSystemPrompt != answerValidationPromptAtV2 {
		t.Errorf("AnswerValidationSystemPrompt が変わった:\n%s", AnswerValidationSystemPrompt)
	}
}

var (
	fieldLinePattern = regexp.MustCompile(`(?m)^- ([a-z_]+): `)
	jsonKeyPattern   = regexp.MustCompile(`"([a-z_]+)":`)
)

// section は見出し（## name）から次の見出しまでを返す。
func section(prompt, name string) string {
	_, rest, ok := strings.Cut(prompt, "## "+name+"\n")
	if !ok {
		return ""
	}
	body, _, _ := strings.Cut(rest, "\n## ")
	return body
}

func TestOutputSchema_プロンプトのキーは構造体と一致する(t *testing.T) {
	// プロンプトの出力形式の節にあるキーを読み、構造体の JSON タグと比べる。
	// プロンプトだけにキーを足す・名前を変えると、ここで失敗する。
	tests := []struct {
		name    string
		section string
		pattern *regexp.Regexp
		output  any
	}{
		{name: "回答品質", section: section(AnswerQualitySystemPrompt, "評価観点"), pattern: fieldLinePattern, output: AnswerQualityOutput{}},
		{name: "回答妥当性", section: section(AnswerValidationSystemPrompt, "出力形式（厳守）"), pattern: jsonKeyPattern, output: AnswerValidationOutput{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.section == "" {
				t.Fatal("出力形式の節が見つからない")
			}
			var inPrompt []string
			for _, m := range tt.pattern.FindAllStringSubmatch(tt.section, -1) {
				if !slices.Contains(inPrompt, m[1]) {
					inPrompt = append(inPrompt, m[1])
				}
			}
			if want := OutputKeys(tt.output); !slices.Equal(inPrompt, want) {
				t.Errorf("プロンプトのキー %v が構造体のキー %v と違う", inPrompt, want)
			}
		})
	}
}

func TestOutputSchema_構造体の変更がプロンプトに出る(t *testing.T) {
	type changed struct {
		Score int    `json:"total" prompt:"総合"`
		Note  string `json:"note" prompt:"メモ"`
	}
	if got := outputFieldList(changed{}); got != "- total: 総合\n- note: メモ" {
		t.Errorf("outputFieldList = %q", got)
	}
	type flag struct {
		OK   bool    `json:"ok"`
		Rate float64 `json:"rate"`
	}
	if got := outputExample(flag{}, false); got != `{"ok": false, "rate": 0.0}` {
		t.Errorf("outputExample = %q", got)
	}
}
