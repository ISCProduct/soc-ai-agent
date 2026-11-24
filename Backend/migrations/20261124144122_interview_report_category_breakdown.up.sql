-- 面接レポートの質問カテゴリ別の評価（SOCAIAGENT-610）。
-- [{category, score, weakness, practice}] の配列。NULL は内訳なし（この列を足す前のレポートを含む）。
ALTER TABLE `interview_reports`
  ADD COLUMN `category_breakdown_json` JSON DEFAULT NULL
  COMMENT '質問カテゴリ別の評価';
