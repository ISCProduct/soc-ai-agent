-- 000003_add_withdrawn_users.up.sql
-- 退会・ユーザーデータ削除機能 (#613)
-- users に論理削除カラムを追加し、退会猶予期間管理テーブルを新設する

ALTER TABLE `users`
  ADD COLUMN `withdrawn_at` datetime(3) DEFAULT NULL,
  ADD INDEX `idx_users_withdrawn_at` (`withdrawn_at`);

CREATE TABLE `withdrawn_users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `email_hash` varchar(64) NOT NULL,
  `email_masked` varchar(255) DEFAULT NULL,
  `reason` varchar(20) NOT NULL,
  `actor_email` varchar(255) DEFAULT NULL,
  `s3_object_keys` text,
  `withdrawn_at` datetime(3) NOT NULL,
  `purge_after` datetime(3) NOT NULL,
  `purged_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_withdrawn_users_user_id` (`user_id`),
  KEY `idx_withdrawn_users_email_hash` (`email_hash`),
  KEY `idx_withdrawn_users_withdrawn_at` (`withdrawn_at`),
  KEY `idx_withdrawn_users_purge_after` (`purge_after`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
