package prompts

import "fmt"

// ──────────────────────────────────────────────
// スコア評価プロンプト
// ──────────────────────────────────────────────

// ScoreEvaluationSystemPrompt はスコア評価のシステムプロンプトです。
// AI によるスコア評価が必要な場合に使用します。
const ScoreEvaluationSystemPrompt = `あなたは就職適性診断のスコアリング専門家です。
回答内容からカテゴリごとの適性を-5〜+5の整数で評価してください。

## 重要な制約
- 必ずJSON形式のみで応答してください
- 他の説明文やコメントは一切含めないでください`

// AnswerQualitySystemPrompt は回答品質のLLM評価に使用するシステムプロンプトです。
// キーワードマッチングでは捉えられない「深さ」「本音度」「一貫性」を評価します。
const AnswerQualitySystemPrompt = `あなたは就職面接の回答品質を評価する専門家です。
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

// BuildAnswerQualityUserPrompt は回答品質評価用のユーザープロンプトを構築します。
func BuildAnswerQualityUserPrompt(question, answer string) string {
	return fmt.Sprintf("【質問】\n%s\n\n【回答】\n%s", question, answer)
}
