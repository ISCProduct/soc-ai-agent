package prompts_test

import (
	"strings"
	"testing"

	"Backend/internal/services/prompts"
)

func TestSystemRoleConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		contains string
	}{
		{"SystemRoleExpert", prompts.SystemRoleExpert, "専門家"},
		{"SystemRoleValidator", prompts.SystemRoleValidator, "審査AI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.constant, tt.contains) {
				t.Errorf("%s に %q が含まれていない: %q", tt.name, tt.contains, tt.constant)
			}
		})
	}
}

func TestCommonConstraintPrompt(t *testing.T) {
	if !strings.Contains(prompts.CommonConstraintPrompt, "重複厳禁") {
		t.Error("CommonConstraintPrompt に「重複厳禁」が含まれていない")
	}
	if !strings.Contains(prompts.CommonConstraintPrompt, "進捗表示禁止") {
		t.Error("CommonConstraintPrompt に「進捗表示禁止」が含まれていない")
	}
}

func TestJobContextGuidance(t *testing.T) {
	if !strings.Contains(prompts.JobContextGuidance, "%s") {
		t.Error("JobContextGuidance に職種名のプレースホルダーがない")
	}
	if !strings.Contains(prompts.JobContextGuidance, "%d") {
		t.Error("JobContextGuidance に業界/職種IDのプレースホルダーがない")
	}
}

func TestPhaseEvaluationPoints(t *testing.T) {
	requiredPhases := []string{"job_analysis", "interest_analysis", "aptitude_analysis", "future_analysis"}
	for _, phase := range requiredPhases {
		t.Run(phase, func(t *testing.T) {
			val, ok := prompts.PhaseEvaluationPoints[phase]
			if !ok {
				t.Errorf("PhaseEvaluationPoints に %q が存在しない", phase)
			}
			if val == "" {
				t.Errorf("PhaseEvaluationPoints[%q] が空文字", phase)
			}
		})
	}
}

func TestRealityCheckGuidelines(t *testing.T) {
	if !strings.Contains(prompts.RealityCheckGuidelines, "本音") {
		t.Error("RealityCheckGuidelines に「本音」が含まれていない")
	}
}

func TestBuildStrategicQuestionPrompt(t *testing.T) {
	result := prompts.BuildStrategicQuestionPrompt(
		"新卒", "就活フェーズ", "選択肢A/B",
		"過去の会話履歴", "スコア分析", "既出質問リスト",
		"カテゴリ評価", "リーダーシップ", "説明文",
		"エンジニア", 1, 2,
	)

	cases := []struct {
		label string
		text  string
	}{
		{"SystemRoleExpert", prompts.SystemRoleExpert},
		{"NewGradGuidelines の一部", "新卒"},
		{"CommonConstraintPrompt の一部", "重複厳禁"},
		{"JobContextGuidance の展開", "エンジニア"},
		{"historyText", "過去の会話履歴"},
		{"scoreAnalysis", "スコア分析"},
		{"targetCategory", "リーダーシップ"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			if !strings.Contains(result, c.text) {
				t.Errorf("BuildStrategicQuestionPrompt の出力に %q が含まれていない", c.text)
			}
		})
	}
}

func TestBuildStrategicQuestionPromptWithPhase(t *testing.T) {
	t.Run("フェーズあり", func(t *testing.T) {
		result := prompts.BuildStrategicQuestionPromptWithPhase(
			"新卒", "job_analysis", "就活フェーズ", "選択肢A/B",
			"会話履歴", "スコア分析", "既出質問",
			"目的", "職種分析", "説明",
			"営業", 3, 4,
		)
		if !strings.Contains(result, "職種分析") {
			t.Error("フェーズの評価観点が含まれていない")
		}
	})

	t.Run("存在しないフェーズ", func(t *testing.T) {
		result := prompts.BuildStrategicQuestionPromptWithPhase(
			"中途", "unknown_phase", "フェーズ", "選択肢",
			"履歴", "スコア", "既出",
			"目的", "カテゴリ", "説明",
			"デザイナー", 5, 6,
		)
		if !strings.Contains(result, "実務経験") {
			t.Error("中途ガイドラインが含まれていない")
		}
	})
}

func TestBuildLowConfidenceQuestionPrompt(t *testing.T) {
	result := prompts.BuildLowConfidenceQuestionPrompt("会話履歴", "前の質問", 1, 2)
	if !strings.Contains(result, prompts.SystemRoleExpert) {
		t.Error("SystemRoleExpert が含まれていない")
	}
	if !strings.Contains(result, "前の質問") {
		t.Error("lastQuestion が含まれていない")
	}
	if !strings.Contains(result, "答えやすい") {
		t.Error("「答えやすい」が含まれていない")
	}
}

func TestBuildUnevaluatedCategoryQuestionPrompt(t *testing.T) {
	result := prompts.BuildUnevaluatedCategoryQuestionPrompt("履歴", "協調性", "チームで働く力", 1, 2)
	if !strings.Contains(result, "協調性") {
		t.Error("targetCategory が含まれていない")
	}
	if !strings.Contains(result, "チームで働く力") {
		t.Error("description が含まれていない")
	}
}

func TestBuildDeepeningQuestionPrompt(t *testing.T) {
	result := prompts.BuildDeepeningQuestionPrompt("履歴", "リーダーシップ", 85, 1, 2)
	if !strings.Contains(result, "リーダーシップ") {
		t.Error("highestCategory が含まれていない")
	}
	if !strings.Contains(result, "85") {
		t.Error("highestScore が含まれていない")
	}
}

func TestBuildSimplifyQuestionPrompt(t *testing.T) {
	q := "あなたのリーダーシップ経験について教えてください"
	result := prompts.BuildSimplifyQuestionPrompt(q)
	if !strings.Contains(result, q) {
		t.Error("元の質問が含まれていない")
	}
	if !strings.Contains(result, "40〜80文字") {
		t.Error("文字数制約が含まれていない")
	}
}

func TestAnswerValidationSystemPrompt(t *testing.T) {
	if !strings.Contains(prompts.AnswerValidationSystemPrompt, prompts.SystemRoleValidator) {
		t.Error("AnswerValidationSystemPrompt に SystemRoleValidator が含まれていない")
	}
	if !strings.Contains(prompts.AnswerValidationSystemPrompt, "JSON") {
		t.Error("AnswerValidationSystemPrompt に JSON 制約が含まれていない")
	}
}

func TestBuildAnswerValidationUserPrompt(t *testing.T) {
	question := "あなたの強みを教えてください"
	answer := "コミュニケーション能力です"
	result := prompts.BuildAnswerValidationUserPrompt(question, answer)

	if !strings.Contains(result, question) {
		t.Error("質問が含まれていない")
	}
	if !strings.Contains(result, answer) {
		t.Error("回答が含まれていない")
	}
	if !strings.Contains(result, `{"valid": true}`) {
		t.Error("判定フォーマットが含まれていない")
	}
}
