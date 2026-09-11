-- 人事ゲスト投稿の連絡先・招待メール状態と企業クレーム（#754）
-- ゲスト投稿 vs クロール vs 本登録: 公開は管理者 publish のみ。クレーム後も draft のまま管理可能。

CREATE TABLE IF NOT EXISTS `company_entry_submissions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `company_id` bigint unsigned NOT NULL,
  `contact_email` varchar(255) NOT NULL,
  `contact_name` varchar(255) NOT NULL DEFAULT '',
  `privacy_consent_at` datetime(3) NOT NULL,
  `source_ip` varchar(64) NOT NULL DEFAULT '',
  `status` varchar(32) NOT NULL DEFAULT 'submitted',
  `email_status` varchar(32) NOT NULL DEFAULT 'pending',
  `email_sent_at` datetime(3) DEFAULT NULL,
  `email_last_error` text,
  `email_attempts` int NOT NULL DEFAULT 0,
  `invite_token` varchar(255) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ces_company_id` (`company_id`),
  KEY `idx_ces_contact_email` (`contact_email`),
  KEY `idx_ces_status` (`status`),
  CONSTRAINT `fk_ces_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `company_ownerships` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `company_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `role` varchar(32) NOT NULL DEFAULT 'owner',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_company_ownerships_company` (`company_id`),
  UNIQUE KEY `uk_company_ownerships_company_user` (`company_id`, `user_id`),
  KEY `idx_company_ownerships_user` (`user_id`),
  CONSTRAINT `fk_company_ownerships_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`),
  CONSTRAINT `fk_company_ownerships_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE `pending_registrations`
  ADD COLUMN `company_id` bigint unsigned DEFAULT NULL AFTER `email`,
  ADD COLUMN `submission_id` bigint unsigned DEFAULT NULL AFTER `company_id`,
  ADD KEY `idx_pending_reg_company_id` (`company_id`),
  ADD KEY `idx_pending_reg_submission_id` (`submission_id`);
