package prompts

import (
	"strings"
	"testing"
)

// ──────────────────────────────────────────────
// common.go のテスト
// ──────────────────────────────────────────────

func TestSystemRoleConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		contains string
	}{
		{"SystemRoleExpert", SystemRoleExpert, "専門家"},
		{"SystemRoleValidator", SystemRoleValidator, "審査AI"},
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
	if !strings.Contains(CommonConstraintPrompt, "重複厳禁") {
		t.Error("CommonConstraintPrompt に「重複厳禁」が含まれていない")
	}
	if !strings.Contains(CommonConstraintPrompt, "進捗表示禁止") {
		t.Error("CommonConstraintPrompt に「進捗表示禁止」が含まれていない")
	}
}

func TestJobContextGuidance(t *testing.T) {
	if !strings.Contains(JobContextGuidance, "%s") {
		t.Error("JobContextGuidance に職種名のプレースホルダーがない")
	}
	if !strings.Contains(JobContextGuidance, "%d") {
		t.Error("JobContextGuidance に業界/職種IDのプレースホルダーがない")
	}
}

func TestPhaseEvaluationPoints(t *testing.T) {
	requiredPhases := []string{"job_analysis", "interest_analysis", "aptitude_analysis", "future_analysis"}
	for _, phase := range requiredPhases {
		t.Run(phase, func(t *testing.T) {
			val, ok := PhaseEvaluationPoints[phase]
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
	if !strings.Contains(RealityCheckGuidelines, "本音") {
		t.Error("RealityCheckGuidelines に「本音」が含まれていない")
	}
}

// ──────────────────────────────────────────────
// question_generation.go のテスト
// ──────────────────────────────────────────────

func TestBuildStrategicQuestionPrompt(t *testing.T) {
	result := BuildStrategicQuestionPrompt(
		"新卒", "就活フェーズ", "選択肢A/B",
		"過去の会話履歴", "スコア分析", "既出質問リスト",
		"カテゴリ評価", "リーダーシップ", "説明文",
		"エンジニア", 1, 2,
	)

	cases := []struct {
		label string
		text  string
	}{
		{"SystemRoleExpert", SystemRoleExpert},
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
		result := BuildStrategicQuestionPromptWithPhase(
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
		result := BuildStrategicQuestionPromptWithPhase(
			"中途", "unknown_phase", "フェーズ", "選択肢",
			"履歴", "スコア", "既出",
			"目的", "カテゴリ", "説明",
			"デザイナー", 5, 6,
		)
		// 中途の場合は MidCareerGuidelines が使われる
		if !strings.Contains(result, "実務経験") {
			t.Error("中途ガイドラインが含まれていない")
		}
	})
}

func TestBuildLowConfidenceQuestionPrompt(t *testing.T) {
	result := BuildLowConfidenceQuestionPrompt("会話履歴", "前の質問", 1, 2)
	if !strings.Contains(result, SystemRoleExpert) {
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
	result := BuildUnevaluatedCategoryQuestionPrompt("履歴", "協調性", "チームで働く力", 1, 2)
	if !strings.Contains(result, "協調性") {
		t.Error("targetCategory が含まれていない")
	}
	if !strings.Contains(result, "チームで働く力") {
		t.Error("description が含まれていない")
	}
}

func TestBuildDeepeningQuestionPrompt(t *testing.T) {
	result := BuildDeepeningQuestionPrompt("履歴", "リーダーシップ", 85, 1, 2)
	if !strings.Contains(result, "リーダーシップ") {
		t.Error("highestCategory が含まれていない")
	}
	if !strings.Contains(result, "85") {
		t.Error("highestScore が含まれていない")
	}
}

func TestBuildSimplifyQuestionPrompt(t *testing.T) {
	q := "あなたのリーダーシップ経験について教えてください"
	result := BuildSimplifyQuestionPrompt(q)
	if !strings.Contains(result, q) {
		t.Error("元の質問が含まれていない")
	}
	if !strings.Contains(result, "40〜80文字") {
		t.Error("文字数制約が含まれていない")
	}
}

// ──────────────────────────────────────────────
// answer_validation.go のテスト
// ──────────────────────────────────────────────

func TestAnswerValidationSystemPrompt(t *testing.T) {
	if !strings.Contains(AnswerValidationSystemPrompt, SystemRoleValidator) {
		t.Error("AnswerValidationSystemPrompt に SystemRoleValidator が含まれていない")
	}
	if !strings.Contains(AnswerValidationSystemPrompt, "JSON") {
		t.Error("AnswerValidationSystemPrompt に JSON 制約が含まれていない")
	}
}

func TestBuildAnswerValidationUserPrompt(t *testing.T) {
	question := "あなたの強みを教えてください"
	answer := "コミュニケーション能力です"
	result := BuildAnswerValidationUserPrompt(question, answer)

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

// ──────────────────────────────────────────────
// builder.go のテスト
// ──────────────────────────────────────────────

func TestInferJobType(t *testing.T) {
	tests := []struct {
		input string
		want  JobType
	}{
		{"エンジニア", JobTypeEngineer},
		{"Webエンジニア", JobTypeEngineer},
		{"開発職", JobTypeEngineer},
		{"SE", JobTypeEngineer},
		{"ITコンサル", JobTypeEngineer},
		{"営業", JobTypeSales},
		{"インサイドセールス", JobTypeSales},
		{"マーケティング", JobTypeMarketing},
		{"広告プランナー", JobTypeMarketing},
		{"デザイナー", JobTypeDesign},
		{"UIデザイン", JobTypeDesign},
		{"総合職", JobTypeGeneral},
		{"人事", JobTypeGeneral},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := InferJobType(tt.input)
			if got != tt.want {
				t.Errorf("InferJobType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetJobTypeConfig(t *testing.T) {
	cfg := GetJobTypeConfig("エンジニア")
	if cfg.Type != JobTypeEngineer {
		t.Errorf("GetJobTypeConfig(エンジニア) type = %q, want エンジニア", cfg.Type)
	}
	if cfg.ToneGuideline == "" {
		t.Error("ToneGuideline が空")
	}
	if cfg.TechFocusNote == "" {
		t.Error("TechFocusNote が空")
	}
}

func TestBuildJobTypeToneGuidance(t *testing.T) {
	tests := []struct {
		jobName  string
		contains string
	}{
		{"エンジニア", "エンジニア"},
		{"営業", "営業"},
		{"マーケティング", "マーケ"},
		{"不明な職種", "一般"},
	}
	for _, tt := range tests {
		t.Run(tt.jobName, func(t *testing.T) {
			result := BuildJobTypeToneGuidance(tt.jobName)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("BuildJobTypeToneGuidance(%q) に %q が含まれていない: %s", tt.jobName, tt.contains, result)
			}
		})
	}
}

func TestBuildSimplifyQuestionPromptWithJobType(t *testing.T) {
	q := "あなたのリーダーシップ経験について教えてください"

	t.Run("エンジニア向けは100文字まで許容", func(t *testing.T) {
		result := BuildSimplifyQuestionPromptWithJobType(q, "エンジニア")
		if !strings.Contains(result, "100文字") {
			t.Error("エンジニア向け文字数制約（100文字）が含まれていない")
		}
		if !strings.Contains(result, q) {
			t.Error("元の質問が含まれていない")
		}
	})

	t.Run("営業向けは80文字まで", func(t *testing.T) {
		result := BuildSimplifyQuestionPromptWithJobType(q, "営業")
		if !strings.Contains(result, "80文字") {
			t.Error("営業向け文字数制約（80文字）が含まれていない")
		}
	})
}

func TestPromptVersion(t *testing.T) {
	if PromptVersion == "" {
		t.Error("PromptVersion が空文字")
	}
}

func TestGetMatchingReasonPromptVersion(t *testing.T) {
	v := GetMatchingReasonPromptVersion()
	if !strings.HasPrefix(v, "matching_reason_") {
		t.Errorf("GetMatchingReasonPromptVersion() = %q, プレフィックス matching_reason_ がない", v)
	}
}

func TestBuildAnswerQualitySystemPromptWithContext(t *testing.T) {
	t.Run("属性なし→基本プロンプトと同一", func(t *testing.T) {
		result := BuildAnswerQualitySystemPromptWithContext("", "")
		if result != AnswerQualitySystemPrompt {
			t.Error("属性なしの場合、基本プロンプトと一致すべき")
		}
	})

	t.Run("学年あり→背景セクション追加", func(t *testing.T) {
		result := BuildAnswerQualitySystemPromptWithContext("新卒", "")
		if !strings.Contains(result, "新卒") {
			t.Error("学年情報が含まれていない")
		}
		if !strings.Contains(result, "背景") {
			t.Error("背景セクションが含まれていない")
		}
	})

	t.Run("学年・職種両方あり", func(t *testing.T) {
		result := BuildAnswerQualitySystemPromptWithContext("中途", "エンジニア")
		if !strings.Contains(result, "中途") {
			t.Error("学年情報が含まれていない")
		}
		if !strings.Contains(result, "エンジニア") {
			t.Error("志望職種が含まれていない")
		}
	})
}
