package prompts

import "fmt"

// ──────────────────────────────────────────────
// 戦略的質問生成プロンプト（generateStrategicQuestion 用）
// ──────────────────────────────────────────────

// buildStrategicQuestionBase は戦略的質問生成の共通ベースを構築します。
func buildStrategicQuestionBase(
	targetLevel, phaseEvalPoint, phaseContext, choiceGuidance,
	historyText, scoreAnalysis, askedQuestionsText,
	questionPurpose, targetCategory, description,
	jobCategoryName string, industryID, jobCategoryID uint,
) string {
	role := SystemRoleExpert
	guidelines := NewGradGuidelines
	if targetLevel == "中途" {
		guidelines = MidCareerGuidelines
	}

	jobContext := fmt.Sprintf(JobContextGuidance, jobCategoryName, industryID, jobCategoryID)

	return fmt.Sprintf(`%s
これまでの会話と評価状況を分析し、**%sが答えやすく、かつ本音を引き出せる質問**を1つ生成してください。

%s
%s
%s
%s
%s
%s

## これまでの会話
%s

%s

%s

## 質問の目的
%s

## 対象カテゴリ: %s
%s

%s

%s`,
		role,
		targetLevel,
		phaseContext,
		choiceGuidance,
		phaseEvalPoint,
		RealityCheckGuidelines,
		guidelines,
		CommonConstraintPrompt,
		historyText,
		scoreAnalysis,
		askedQuestionsText,
		questionPurpose,
		targetCategory,
		description,
		jobContext,
		CommonConstraintPrompt)
}

// BuildStrategicQuestionPrompt は戦略的質問生成用のプロンプトを構築します。
func BuildStrategicQuestionPrompt(
	targetLevel, phaseContext, choiceGuidance,
	historyText, scoreAnalysis, askedQuestionsText,
	questionPurpose, targetCategory, description,
	jobCategoryName string, industryID, jobCategoryID uint,
) string {
	return buildStrategicQuestionBase(
		targetLevel, "", phaseContext, choiceGuidance,
		historyText, scoreAnalysis, askedQuestionsText,
		questionPurpose, targetCategory, description,
		jobCategoryName, industryID, jobCategoryID,
	)
}

// BuildStrategicQuestionPromptWithPhase はフェーズ名を受け取り、評価観点を含む戦略的質問プロンプトを構築します。
func BuildStrategicQuestionPromptWithPhase(
	targetLevel, phaseName, phaseContext, choiceGuidance,
	historyText, scoreAnalysis, askedQuestionsText,
	questionPurpose, targetCategory, description,
	jobCategoryName string, industryID, jobCategoryID uint,
) string {
	phaseEvalPoint := ""
	if ep, ok := PhaseEvaluationPoints[phaseName]; ok {
		phaseEvalPoint = ep + "\n"
	}

	return buildStrategicQuestionBase(
		targetLevel, phaseEvalPoint, phaseContext, choiceGuidance,
		historyText, scoreAnalysis, askedQuestionsText,
		questionPurpose, targetCategory, description,
		jobCategoryName, industryID, jobCategoryID,
	)
}

// ──────────────────────────────────────────────
// フォールバック質問生成プロンプト（generateQuestionWithAI 用）
// ──────────────────────────────────────────────

// BuildLowConfidenceQuestionPrompt は「わからない」系の回答後の再質問プロンプトを構築します。
func BuildLowConfidenceQuestionPrompt(historyText, lastQuestion string, industryID, jobCategoryID uint) string {
	jobContext := fmt.Sprintf(JobContextGuidance, "不明", industryID, jobCategoryID)
	return fmt.Sprintf(`%s

## これまでの会話
%s

## 状況
学生が前の質問「%s」に答えられなかったようです。
同じカテゴリで、**より答えやすい質問**を生成してください。

%s
%s
%s

%s`, SystemRoleExpert, historyText, lastQuestion, NewGradGuidelines, RealityCheckGuidelines, jobContext, CommonConstraintPrompt)
}

// BuildUnevaluatedCategoryQuestionPrompt は未評価カテゴリに対する質問プロンプトを構築します。
func BuildUnevaluatedCategoryQuestionPrompt(historyText, targetCategory, description string, industryID, jobCategoryID uint) string {
	jobContext := fmt.Sprintf(JobContextGuidance, "不明", industryID, jobCategoryID)
	return fmt.Sprintf(`%s

## これまでの会話
%s

## 次に評価すべきカテゴリ
**%s** (%s)

%s
%s
%s

%s`, SystemRoleExpert, historyText, targetCategory, description, NewGradGuidelines, RealityCheckGuidelines, jobContext, CommonConstraintPrompt)
}

// BuildDeepeningQuestionPrompt は全カテゴリ評価済み後の深掘り質問プロンプトを構築します。
func BuildDeepeningQuestionPrompt(historyText, highestCategory string, highestScore int, industryID, jobCategoryID uint) string {
	jobContext := fmt.Sprintf(JobContextGuidance, "不明", industryID, jobCategoryID)
	return fmt.Sprintf(`%s

## これまでの会話
%s

## 現在の評価状況
学生の強みとして「%s」が見えてきました（スコア: %d）。
この強みを深掘りし、具体的なエピソードや考え方を引き出す質問を作成してください。

%s
%s
%s
%s

%s`, SystemRoleExpert, historyText, highestCategory, highestScore, NewGradGuidelines, RealityCheckGuidelines, DeepeningGuidelines, jobContext, CommonConstraintPrompt)
}

// ──────────────────────────────────────────────
// 質問簡略化プロンプト（simplifyQuestionWithAI 用）
// ──────────────────────────────────────────────

// BuildSimplifyQuestionPrompt は質問簡略化用のプロンプトを構築します。
// 元の意図を保ちながら短く言い換えるよう、自己検証ステップを含みます。
func BuildSimplifyQuestionPrompt(question string) string {
	return fmt.Sprintf(`次の質問を、新卒でも答えやすい短い質問に言い換えてください。

## 制約
- 1文で、40〜80文字程度
- 例示やカッコ補足は入れない
- 元の質問の意図・キーワードを必ず保持する
- 質問文のみを返す

## 自己検証
言い換えた質問が以下を満たすか確認してから出力してください：
1. 元の質問が問いたい「評価対象（技術志向・リーダーシップ等）」が伝わるか
2. 新卒学生が学生生活の経験で答えられる内容か
3. 40〜80文字の範囲に収まっているか

質問:
%s`, question)
}

// BuildSimplifyQuestionPromptWithJobType は職種別のトーンと文字数ガイドラインを考慮した
// 質問簡略化プロンプトを構築します。
func BuildSimplifyQuestionPromptWithJobType(question, jobCategoryName string) string {
	cfg := GetJobTypeConfig(jobCategoryName)
	charRange := "40〜80文字"
	// エンジニア向けは技術的な文脈を保つため若干長めも許容
	if cfg.Type == JobTypeEngineer {
		charRange = "40〜100文字"
	}
	return fmt.Sprintf(`次の質問を、%s向けに短く言い換えてください。

## 制約
- 1文で、%s程度
- 例示やカッコ補足は入れない
- 元の質問の意図・キーワードを必ず保持する
- 質問文のみを返す
- %s

## 自己検証
言い換えた質問が以下を満たすか確認してから出力してください：
1. 元の質問が問いたい「評価対象（技術志向・リーダーシップ等）」が伝わるか
2. 対象者が経験をもとに答えられる内容か
3. 文字数の範囲に収まっているか

質問:
%s`, jobCategoryName, charRange, cfg.TechFocusNote, question)
}
