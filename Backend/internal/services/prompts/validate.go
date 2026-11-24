package prompts

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

// 宣言済みの版（SOCAIAGENT-661）。
//
// PROMPT_VERSION / PROMPT_VERSION_<STAGE> は「今の本文」に付けるラベルで、本文はコードにある。
// 本文の無い版を指定すると、本文は今のまま版ラベルだけが変わり、使用量ログの版が実際の本文を
// 指さなくなる。そこで、環境変数で指定できる版をここで宣言したものに限り、起動時に確かめる。
//
// 本文を変えたら、その工程の版をここで書き換え（または足し）、同じリリースで環境変数も揃える。
// A/B の候補の版（RegisterCandidateBodies）は PROMPT_AB_<STAGE> でだけ使い、ここには入れない。
// PROMPT_VERSION_<STAGE> で候補の版を選んでも Body() は今の本文を返すため。
var declaredPromptVersions = map[Stage][]string{
	StageQuestionGeneration: {defaultPromptVersion},
	StageScoreEvaluation:    {defaultPromptVersion},
	StageMatchingReason:     {defaultPromptVersion},
	StageCompanyExtraction:  {defaultPromptVersion},
}

// promptStages は環境変数で版を指定できる工程（knownPromptStage と同じ並び）。
var promptStages = []Stage{StageQuestionGeneration, StageScoreEvaluation, StageMatchingReason, StageCompanyExtraction}

// DeclaredPromptVersions は工程で宣言済みの版を返す。
func DeclaredPromptVersions(stage Stage) []string {
	return slices.Clone(declaredPromptVersions[stage])
}

// ValidateSettings はプロンプト版の環境変数が宣言済みの版を指しているかを確かめる。
// 起動時に1回だけ呼び、エラーなら起動を止める。どの環境変数が不正かをすべて返す。
//
// 環境変数の値は形式（ValidPromptVersion）に合うときだけメッセージに含める。
// 形式に合わない値は、誤って別の設定（シークレットを含みうる）を入れた可能性があるため出さない。
func ValidateSettings() error {
	var errs []error
	globalKey := "PROMPT_VERSION"
	global := strings.TrimSpace(os.Getenv(globalKey))
	if global != "" && !ValidPromptVersion(global) {
		errs = append(errs, fmt.Errorf("%s: 版の形式が不正です（英数字で始まり、英数字・.・_・- のみの最大64文字）", globalKey))
		global = ""
	}
	var globalUndeclared []string
	for _, stage := range promptStages {
		key := "PROMPT_VERSION_" + strings.ToUpper(string(stage))
		value := strings.TrimSpace(os.Getenv(key))
		declared := declaredPromptVersions[stage]
		switch {
		case value != "" && !ValidPromptVersion(value):
			errs = append(errs, fmt.Errorf("%s: 版の形式が不正です（英数字で始まり、英数字・.・_・- のみの最大64文字）", key))
		case value != "":
			if !slices.Contains(declared, value) {
				errs = append(errs, fmt.Errorf("%s=%s: 宣言されていない版です（宣言済み: %s）", key, value, strings.Join(declared, ", ")))
			}
		case global != "":
			// 工程別の指定が無い工程には、共通の PROMPT_VERSION が効く
			if !slices.Contains(declared, global) {
				globalUndeclared = append(globalUndeclared, string(stage))
			}
		}
	}
	if len(globalUndeclared) > 0 {
		errs = append(errs, fmt.Errorf("%s=%s: 工程 %s で宣言されていない版です（工程別の PROMPT_VERSION_<STAGE> が無い工程に効きます）",
			globalKey, global, strings.Join(globalUndeclared, ", ")))
	}
	if len(errs) > 0 {
		return fmt.Errorf("プロンプト版の設定が不正です: %w", errors.Join(errs...))
	}
	return nil
}
