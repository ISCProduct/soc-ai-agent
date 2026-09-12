-- マッチング結果に (user_id, session_id, company_id) の一意制約を張る（#1166 / SOCAIAGENT-304）
--
-- CreateOrUpdateBatch は既存行を1件ずつ UPDATE していたため、再計算時に公開企業数ぶんの
-- クエリが走っていた。ON DUPLICATE KEY UPDATE による一括 upsert へ寄せるには、
-- 衝突を検出する一意キーが必要になる。
--
-- 従来の実装は「読んでから作る」方式で一意制約が無かったため、同時実行で重複行が
-- できている可能性がある。ALTER が失敗しないよう、先に重複を1行へ集約する。
-- 集約時は閲覧/お気に入り/応募フラグを失わないよう、重複行の値を OR（MAX）で寄せる。

UPDATE `user_company_matches` t
JOIN (
  SELECT MIN(`id`) AS keep_id,
         MAX(`is_viewed`)    AS is_viewed,
         MAX(`is_favorited`) AS is_favorited,
         MAX(`is_applied`)   AS is_applied
    FROM `user_company_matches`
   GROUP BY `user_id`, `session_id`, `company_id`
  HAVING COUNT(*) > 1
) d ON t.`id` = d.keep_id
SET t.`is_viewed`    = d.is_viewed,
    t.`is_favorited` = d.is_favorited,
    t.`is_applied`   = d.is_applied;

DELETE t FROM `user_company_matches` t
JOIN (
  SELECT `user_id`, `session_id`, `company_id`, MIN(`id`) AS keep_id
    FROM `user_company_matches`
   GROUP BY `user_id`, `session_id`, `company_id`
  HAVING COUNT(*) > 1
) d
  ON t.`user_id` = d.`user_id`
 AND t.`session_id` = d.`session_id`
 AND t.`company_id` = d.`company_id`
WHERE t.`id` <> d.keep_id;

ALTER TABLE `user_company_matches`
  ADD UNIQUE KEY `uniq_user_session_company` (`user_id`, `session_id`, `company_id`);
