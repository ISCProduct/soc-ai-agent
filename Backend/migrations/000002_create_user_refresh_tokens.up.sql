-- 000002_create_user_refresh_tokens.up.sql
-- リフレッシュトークン管理テーブル (#616)
-- トークン本体は保存せず SHA-256 ハッシュのみ保持する

CREATE TABLE `user_refresh_tokens` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `token_hash` varchar(64) NOT NULL,
  `expires_at` datetime(3) NOT NULL,
  `revoked_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_user_refresh_tokens_token_hash` (`token_hash`),
  KEY `idx_user_refresh_tokens_user_id` (`user_id`),
  KEY `idx_user_refresh_tokens_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
