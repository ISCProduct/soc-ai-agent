-- スカウト送信と定型文テンプレート（#1095）

CREATE TABLE IF NOT EXISTS `scout_templates` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `company_id` bigint unsigned NOT NULL,
  `title` varchar(80) NOT NULL,
  `body` text NOT NULL,
  `created_by` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_scout_templates_company` (`company_id`),
  CONSTRAINT `fk_scout_templates_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_scout_templates_creator` FOREIGN KEY (`created_by`) REFERENCES `company_users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `scouts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `company_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `template_id` bigint unsigned DEFAULT NULL,
  `sent_by` bigint unsigned NOT NULL,
  `message` text NOT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'sent',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_scouts_company_created` (`company_id`, `created_at`),
  KEY `idx_scouts_user_created` (`user_id`, `created_at`),
  KEY `idx_scouts_company_user_created` (`company_id`, `user_id`, `created_at`),
  CONSTRAINT `fk_scouts_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_scouts_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_scouts_template` FOREIGN KEY (`template_id`) REFERENCES `scout_templates` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_scouts_sender` FOREIGN KEY (`sent_by`) REFERENCES `company_users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 学生が企業からのスカウトを受け取らない設定（ブロック）
CREATE TABLE IF NOT EXISTS `scout_company_blocks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `company_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_scout_company_blocks` (`user_id`, `company_id`),
  KEY `idx_scout_company_blocks_company` (`company_id`),
  CONSTRAINT `fk_scout_company_blocks_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_scout_company_blocks_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
