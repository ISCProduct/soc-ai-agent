-- 企業研究AIの材料のうち、数字（SOCAIAGENT-722 / 724）。
-- 数字を1項目ずつ、出典と対象年度つきで持つ。「どの年度の誰の数字か」を答えに添えるため。
-- 同じ項目でも情報源ごとに1行（EDINET と gBizINFO で値が違えば両方残す）。
CREATE TABLE `company_facts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `company_id` bigint unsigned NOT NULL,
  -- employees_consolidated / employees_single / average_age / average_service_years / average_salary / net_sales など
  `field` varchar(64) NOT NULL,
  `value` varchar(255) NOT NULL,
  -- 人 / 歳 / 年 / 円 など。値の解釈に必要なので必ず入れる
  `unit` varchar(16) NOT NULL DEFAULT '',
  `source_type` varchar(32) NOT NULL,
  `source_url` varchar(500) NOT NULL DEFAULT '',
  `as_of` date DEFAULT NULL,
  `fetched_at` datetime(3) NOT NULL,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_company_facts_field_source` (`company_id`, `field`, `source_type`),
  CONSTRAINT `fk_company_facts_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
