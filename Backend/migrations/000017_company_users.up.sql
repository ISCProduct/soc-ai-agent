-- 企業向けポータル用アカウント（#1091）
CREATE TABLE IF NOT EXISTS `company_users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `company_id` bigint unsigned NOT NULL,
  `email` varchar(255) NOT NULL,
  `password` varchar(255) NOT NULL DEFAULT '',
  `name` varchar(255) NOT NULL DEFAULT '',
  `role` varchar(32) NOT NULL DEFAULT 'member',
  `invite_token` varchar(255) DEFAULT NULL,
  `invite_expires_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_company_users_email` (`email`),
  KEY `idx_company_users_company_id` (`company_id`),
  CONSTRAINT `fk_company_users_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `company_user_refresh_tokens` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `company_user_id` bigint unsigned NOT NULL,
  `token_hash` varchar(64) NOT NULL,
  `expires_at` datetime(3) NOT NULL,
  `revoked_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_company_user_refresh_token_hash` (`token_hash`),
  KEY `idx_company_user_refresh_tokens_user` (`company_user_id`),
  KEY `idx_company_user_refresh_tokens_expires` (`expires_at`),
  CONSTRAINT `fk_company_user_refresh_tokens_user` FOREIGN KEY (`company_user_id`) REFERENCES `company_users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
