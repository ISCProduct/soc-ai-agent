package prompts

import (
	"reflect"
	"strings"
)

// LLM の出力スキーマ（SOCAIAGENT-661）。
//
// プロンプトに書く出力形式と、出力を読む Go の構造体が別々に書かれていると、片方だけを
// 直したときにずれる（キーを足しても読まれない、名前を変えるとゼロ値で通る）。
// 構造体を正とし、プロンプトの出力形式はここから作る。キーを変えるときは構造体のタグを直す。
// プロンプトの本文が変わるので、その工程の版（declaredPromptVersions）も同じ変更で上げる。

// AnswerValidationOutput は回答妥当性チェック（AnswerValidationSystemPrompt）の出力。
type AnswerValidationOutput struct {
	Valid      bool    `json:"valid"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

// AnswerQualityOutput は回答品質の LLM 評価（AnswerQualitySystemPrompt）の出力。
// prompt タグはプロンプトの「評価観点」に並べる説明。
type AnswerQualityOutput struct {
	Score        int    `json:"score" prompt:"総合スコア（0〜100の整数）"`
	Confidence   string `json:"confidence" prompt:"評価の信頼度 \"high\" / \"medium\" / \"low\""`
	Specificity  int    `json:"specificity" prompt:"具体性（0〜3）- 具体的な経験・数値・エピソードの有無"`
	Authenticity int    `json:"authenticity" prompt:"本音度（0〜3）- 定型文でなく個人の実感が感じられるか"`
	Consistency  int    `json:"consistency" prompt:"一貫性（0〜3）- 論理的で矛盾がないか"`
	Explanation  string `json:"explanation" prompt:"評価理由（日本語、60文字以内）"`
}

// OutputKeys は出力構造体の JSON キーを、フィールドの順に返す。
func OutputKeys(v any) []string {
	t := reflect.TypeOf(v)
	keys := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			keys = append(keys, name)
		}
	}
	return keys
}

// outputFieldList は「- キー: 説明」の行を、フィールドの順に並べる。
func outputFieldList(v any) string {
	t := reflect.TypeOf(v)
	lines := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		lines = append(lines, "- "+name+": "+f.Tag.Get("prompt"))
	}
	return strings.Join(lines, "\n")
}

// outputExample は出力の例（1行の JSON）を作る。bool は boolValue、文字列は "..."、数値は 0.0。
func outputExample(v any, boolValue bool) string {
	t := reflect.TypeOf(v)
	parts := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		var value string
		switch f.Type.Kind() {
		case reflect.Bool:
			value = "false"
			if boolValue {
				value = "true"
			}
		case reflect.String:
			value = `"..."`
		default:
			value = "0.0"
		}
		parts = append(parts, `"`+name+`": `+value)
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
