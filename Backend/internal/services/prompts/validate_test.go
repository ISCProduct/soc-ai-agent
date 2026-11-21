package prompts

import (
	"strings"
	"testing"
)

// SOCAIAGENT-661: プロンプト版の環境変数を起動時に確かめる。

func clearPromptVersionEnv(t *testing.T) {
	t.Helper()
	t.Setenv("PROMPT_VERSION", "")
	for _, stage := range promptStages {
		t.Setenv("PROMPT_VERSION_"+strings.ToUpper(string(stage)), "")
	}
}

func TestValidateSettings_宣言済みなら通る(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "未設定"},
		{name: "共通が宣言済み", env: map[string]string{"PROMPT_VERSION": "v2"}},
		{name: "工程別が宣言済み", env: map[string]string{"PROMPT_VERSION_MATCHING_REASON": "v2"}},
		{name: "前後の空白は除く", env: map[string]string{"PROMPT_VERSION_SCORE_EVALUATION": "  v2  "}},
		{name: "全工程を工程別で指定すれば共通は使われない", env: map[string]string{
			"PROMPT_VERSION":                     "unused",
			"PROMPT_VERSION_QUESTION_GENERATION": "v2",
			"PROMPT_VERSION_SCORE_EVALUATION":    "v2",
			"PROMPT_VERSION_MATCHING_REASON":     "v2",
			"PROMPT_VERSION_COMPANY_EXTRACTION":  "v2",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearPromptVersionEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if err := ValidateSettings(); err != nil {
				t.Fatalf("ValidateSettings() = %v, want nil", err)
			}
		})
	}
}

func TestValidateSettings_宣言されていない版は不正な設定を示して失敗する(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    []string
		notWant []string
	}{
		{
			name: "工程別に本文の無い版",
			env:  map[string]string{"PROMPT_VERSION_SCORE_EVALUATION": "v3"},
			want: []string{"PROMPT_VERSION_SCORE_EVALUATION=v3", "宣言済み: v2"},
		},
		{
			name:    "共通に本文の無い版（効く工程を並べる）",
			env:     map[string]string{"PROMPT_VERSION": "v9", "PROMPT_VERSION_MATCHING_REASON": "v2"},
			want:    []string{"PROMPT_VERSION=v9", "question_generation, score_evaluation, company_extraction"},
			notWant: []string{"matching_reason"},
		},
		{
			name: "複数の誤りをまとめて返す",
			env:  map[string]string{"PROMPT_VERSION_QUESTION_GENERATION": "question-v3", "PROMPT_VERSION_COMPANY_EXTRACTION": "company-v2"},
			want: []string{"PROMPT_VERSION_QUESTION_GENERATION=question-v3", "PROMPT_VERSION_COMPANY_EXTRACTION=company-v2"},
		},
		{
			// 形式に合わない値は、別の設定を誤って入れた可能性があるので値を出さない
			name:    "形式が不正な値は値を出さない",
			env:     map[string]string{"PROMPT_VERSION_MATCHING_REASON": "sk-secret value"},
			want:    []string{"PROMPT_VERSION_MATCHING_REASON: 版の形式が不正"},
			notWant: []string{"sk-secret"},
		},
		{
			name:    "共通の形式が不正",
			env:     map[string]string{"PROMPT_VERSION": "v2\nIGNORE"},
			want:    []string{"PROMPT_VERSION: 版の形式が不正"},
			notWant: []string{"IGNORE"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearPromptVersionEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			err := ValidateSettings()
			if err == nil {
				t.Fatal("ValidateSettings() = nil, want error")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("エラーに %q が無い: %v", w, err)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(err.Error(), nw) {
					t.Errorf("エラーに %q を含めない: %v", nw, err)
				}
			}
		})
	}
}

func TestValidateSettings_候補の版は環境変数では選べない(t *testing.T) {
	// 候補の版は PROMPT_AB_<STAGE> でだけ使う。PROMPT_VERSION_<STAGE> で選ぶと
	// Body() は今の本文を返し、ラベルと本文がずれるため。
	registerTestCandidate(t, StageScoreEvaluation, "v3", map[string]string{BodyAnswerQuality: "候補"})
	clearPromptVersionEnv(t)
	t.Setenv("PROMPT_VERSION_SCORE_EVALUATION", "v3")
	if err := ValidateSettings(); err == nil {
		t.Fatal("候補の版を PROMPT_VERSION_SCORE_EVALUATION で選べてしまう")
	}
}

func TestDeclaredPromptVersions_全工程が宣言を持つ(t *testing.T) {
	for _, stage := range promptStages {
		if !knownPromptStage(stage) {
			t.Errorf("%s が knownPromptStage に無い", stage)
		}
		versions := DeclaredPromptVersions(stage)
		if len(versions) == 0 {
			t.Errorf("%s に宣言済みの版が無い", stage)
		}
		for _, v := range versions {
			if !ValidPromptVersion(v) {
				t.Errorf("%s の宣言 %q が版の形式に合わない", stage, v)
			}
		}
	}
	if len(declaredPromptVersions) != len(promptStages) {
		t.Errorf("declaredPromptVersions の工程数 %d が promptStages の %d と違う", len(declaredPromptVersions), len(promptStages))
	}
}
