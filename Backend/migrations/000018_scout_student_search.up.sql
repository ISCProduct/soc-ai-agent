-- スカウト機能: 学生の希望条件と企業ごとの学生タグ（#1094）

-- 学生の希望条件。企業側のフィルタ軸（希望業界・希望勤務地）を保持する。
-- 既存 users テーブルには該当項目が無いため新設（1学生1レコード）。
CREATE TABLE IF NOT EXISTS `user_preferences` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `desired_industry_id` bigint unsigned DEFAULT NULL,
  `desired_location` varchar(100) NOT NULL DEFAULT '',
  `note` text,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_preferences_user` (`user_id`),
  KEY `idx_user_preferences_industry` (`desired_industry_id`),
  KEY `idx_user_preferences_location` (`desired_location`),
  CONSTRAINT `fk_user_preferences_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_user_preferences_industry` FOREIGN KEY (`desired_industry_id`) REFERENCES `industries` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 企業ごとの学生タグ。company_id で完全に分離し、他社のタグは見えない。
CREATE TABLE IF NOT EXISTS `company_student_tags` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `company_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `tag_name` varchar(64) NOT NULL,
  `created_by` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_company_student_tags` (`company_id`,`user_id`,`tag_name`),
  KEY `idx_company_student_tags_company_tag` (`company_id`,`tag_name`),
  KEY `idx_company_student_tags_user` (`user_id`),
  CONSTRAINT `fk_company_student_tags_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_company_student_tags_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_company_student_tags_creator` FOREIGN KEY (`created_by`) REFERENCES `company_users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
