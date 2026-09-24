-- 企業→学校の掲載申請（#1506）。
-- 企業が「自校の学生に掲載したい」と申請し、学校のキャリア担当が承認する。
-- 承認されたら既存の school_company_approvals に行を作る（#1507）。
CREATE TABLE `school_company_applications` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `school_id` bigint unsigned NOT NULL,
  `company_id` bigint unsigned NOT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'pending',
  `applied_by` bigint unsigned NOT NULL,
  `reviewed_by` bigint unsigned DEFAULT NULL,
  `note` text,
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_sca_school_status` (`school_id`, `status`),
  KEY `idx_sca_company` (`company_id`),
  CONSTRAINT `fk_sca_school` FOREIGN KEY (`school_id`) REFERENCES `schools` (`id`),
  CONSTRAINT `fk_sca_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
