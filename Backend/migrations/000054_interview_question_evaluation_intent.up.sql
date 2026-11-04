-- 面接カスタム質問に「評価したい観点（意図）」を追加する（#1092 / SOCAIAGENT-284）。
--
-- AI 面接官の system プロンプトへ、質問と同じ非信頼テキストの囲みの中で渡す。
-- 任意項目なので NULL を許す（NULL = 未設定）。上限300字はアプリ側で検証する。
ALTER TABLE `interview_company_questions`
  ADD COLUMN `evaluation_intent` TEXT NULL AFTER `question_text`;
