package prompts

import "fmt"

// ──────────────────────────────────────────────
// 回答妥当性チェックプロンプト（validateAnswerRelevance 用）
// ──────────────────────────────────────────────

// AnswerValidationSystemPrompt は回答妥当性チェックのシステムプロンプトです。
const AnswerValidationSystemPrompt = SystemRoleValidator + `

## 重要な制約
- 必ずJSON形式のみで応答してください
- 他の説明文やコメントは一切含めないでください

## 出力形式（厳守）
{"valid": true} または {"valid": false}`

// BuildAnswerValidationUserPrompt は回答妥当性チェック用のユーザープロンプトを構築します。
// 「無効条件のみ列挙」ではなく「有効条件を正面から定義」する方式で、
// 質問との文脈的関連性を判定基準に加えています。
func BuildAnswerValidationUserPrompt(question, answer string) string {
	return fmt.Sprintf(`以下の質問に対するユーザーの回答が適切かどうかを判定してください。

## 質問
%s

## ユーザーの回答
%s

## 有効な回答の条件（以下のいずれか1つを満たせば有効）
1. 選択肢記号（A、B、C、1、2、3など）が含まれている
2. 質問のキーワードや主題に対して何らかの言及がある
3. 自分の経験・考え・好みを示す表現がある（「〜した」「〜が好き」「〜思う」など）
4. 選択肢や例示に対する明確な反応がある
5. 「はい」「いいえ」などの意思表示

## 無効な回答（以下の**すべて**に該当する場合のみ無効）
- 質問の主題に一切触れていない
- かつ 10文字未満（挨拶・短い感嘆詞のみ）、または完全に無関係な話題

## 判定
{"valid": true} または {"valid": false}`, question, answer)
}
