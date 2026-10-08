-- #1529 の取り消し。
--
-- 失われるもの: 項目別スコアの内訳と、「スコア無し」の区別。
-- NOT NULL へ戻す前に NULL を 0 で埋めるため、スコア無しのレビューは
-- 0点（最低評価）として残る。down 後にコードだけ戻しても復元できない。
UPDATE `resume_reviews` SET `score` = 0 WHERE `score` IS NULL;

ALTER TABLE `resume_reviews`
  MODIFY COLUMN `score` bigint NOT NULL DEFAULT '0',
  DROP COLUMN `item_scores_json`;
