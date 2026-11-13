-- 面接の質問計画に、その質問が狙う企業の人物像の軸を記録する（#2274 / SOCAIAGENT-650）。
-- 軸スロット（source=axis）とその深掘りだけに入る。標準トピック・カスタム質問は NULL。
-- 採点には使わない（質問の選択と、あとから質問の偏りを確認するためだけ）。
ALTER TABLE `interview_question_states`
  ADD COLUMN `target_axis` VARCHAR(32) NULL AFTER `category`;
