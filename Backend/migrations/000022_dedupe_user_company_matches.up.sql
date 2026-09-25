-- 重複マッチ行の集約（#1166 / SOCAIAGENT-304）
--
-- 000023 で (user_id, session_id, company_id) に一意キーを張るための事前データ移行。
-- 規約どおりデータ移行とスキーマ変更は別ファイルに分けている(docs/wiki/migrations.md)。
--
-- 従来の CreateOrUpdate/CreateOrUpdateBatch は「読んでから作る」方式で一意制約が
-- 無かったため、同時実行で重複行ができている可能性がある。
--
-- 適用前に重複件数を確認すること（0件なら 1〜4 はすべて 0 行に作用する）:
--   SELECT COUNT(*) FROM (
--     SELECT 1 FROM user_company_matches
--      GROUP BY user_id, session_id, company_id HAVING COUNT(*) > 1) x;

-- 1. session_id の NULL を空文字へ寄せる。
--    Go 側は models.UserCompanyMatch.SessionID が非ポインタ string のため新規に NULL は
--    入らないが、NULL が残っていると
--      (a) 下の集約 JOIN が NULL 同士で一致せず重複が消えない
--      (b) MySQL の UNIQUE は複数 NULL を許すため一意キーが穴になり、
--          ON DUPLICATE KEY UPDATE が衝突せず再計算ごとに行が増え続ける
--    という2つの問題が起きる。
UPDATE `user_company_matches` SET `session_id` = '' WHERE `session_id` IS NULL;

-- 2. 残す行(MIN(id))へ閲覧/お気に入り/応募フラグを寄せる。
--    これらはユーザー操作の結果なので、重複削除で失わないようにする。
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

-- 3. 応募・選考ステータスの参照を残す行へ付け替える。
--    user_application_statuses.match_id に FK は無いため、付け替えずに削除すると参照が
--    dangling になる。参照側はすべて INNER JOIN (user_application_status_repository.go,
--    profile_recalculation_repository.go) なので、応募が一覧・選考ステータス・
--    フライホイールの再計算母集団から黙って消える。
UPDATE `user_application_statuses` a
JOIN `user_company_matches` t ON t.`id` = a.`match_id`
JOIN (
  SELECT `user_id`, `session_id`, `company_id`, MIN(`id`) AS keep_id
    FROM `user_company_matches`
   GROUP BY `user_id`, `session_id`, `company_id`
  HAVING COUNT(*) > 1
) d
  ON d.`user_id` = t.`user_id`
 AND d.`session_id` = t.`session_id`
 AND d.`company_id` = t.`company_id`
SET a.`match_id` = d.keep_id
WHERE a.`match_id` <> d.keep_id;

-- 4. 重複行を削除する。
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
