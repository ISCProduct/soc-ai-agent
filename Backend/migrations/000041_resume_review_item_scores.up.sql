-- 履歴書レビューにルーブリックの項目別スコアを持たせる（#1529）。
--
-- item_scores_json: 項目キー(specificity 等) → 0〜5 の整数。
--   既存行は NULL のまま（画面は「内訳なし」に倒す）。
-- score: 総合スコアを NULL 許容にする。
--   生成失敗・ルーブリック違反のときに固定値70を保存せず「スコア無し」で残すため。
--   0 で表すことはできない（0点は画面に最低評価として表示される）。
ALTER TABLE `resume_reviews`
  ADD COLUMN `item_scores_json` json DEFAULT NULL AFTER `score`,
  MODIFY COLUMN `score` bigint DEFAULT NULL;
