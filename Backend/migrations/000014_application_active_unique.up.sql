-- 同一ユーザー・同一企業について「進行中の応募」は1件のみとする（#1017）。
-- MySQLは部分UNIQUEインデックスを直接サポートしないため、終了状態
-- (withdrawn/rejected/accepted) では NULL になる生成列を使い、
-- 「NULLは重複可・非NULLはUNIQUE」というMySQLのインデックス挙動を利用する。
ALTER TABLE `user_application_statuses`
  ADD COLUMN `active_dedup_key` VARCHAR(64) GENERATED ALWAYS AS (
    CASE WHEN `status` NOT IN ('withdrawn', 'rejected', 'accepted')
      THEN CONCAT(`user_id`, '-', `company_id`)
      ELSE NULL
    END
  ) STORED,
  ADD UNIQUE KEY `uniq_user_application_statuses_active` (`active_dedup_key`);
