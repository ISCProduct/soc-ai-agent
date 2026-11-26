-- 企業の事業タグ（SOCAIAGENT-717）。相関図で分野から企業を探すために使う。
-- 業種(industry)とは別物。募集職種・採用有無も表さない。
-- name は utf8mb4_bin。unicode_ci だと「ハ」と「パ」のような別の語が同一視され、一意制約で登録できなくなる。
-- 全角・半角の統一はアプリ側(NFKC)で行う。
CREATE TABLE IF NOT EXISTS `business_tags` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_business_tags_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `company_business_tags` (
  `company_id` bigint unsigned NOT NULL,
  `business_tag_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`company_id`,`business_tag_id`),
  KEY `idx_company_business_tags_tag` (`business_tag_id`),
  CONSTRAINT `fk_company_business_tags_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_company_business_tags_tag` FOREIGN KEY (`business_tag_id`) REFERENCES `business_tags` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
