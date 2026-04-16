package prompts

const MatchingReasonPromptVersion = "matching_reason_v1"

const MatchingReasonSystemPrompt = `あなたは新卒向け採用マッチングのキャリアアドバイザーです。
与えられたユーザー適性と企業情報を使って、マッチング理由を日本語で作成してください。

制約:
- 2〜3文、120〜220文字程度
- 総合スコアと一致している強み軸を自然に言及
- 企業側の情報（事業・文化・働き方のいずれか）を1つ以上反映
- 断定しすぎず、就活生に寄り添う語調
- 出力は本文のみ（見出し・箇条書き・JSON禁止）`

const MatchingReasonUserPromptTemplate = `【PromptVersion】
%s

【企業情報】
- 企業名: %s
- 業界: %s
- 主事業: %s
- 文化: %s
- 働き方: %s
- 開発スタイル: %s
- 技術スタック: %s

【ユーザー適性（上位3）】
%s

【この企業との一致軸（上位3）】
%s

【総合マッチ度】
%.1f

上記の情報だけを根拠に、マッチング理由を作成してください。`
