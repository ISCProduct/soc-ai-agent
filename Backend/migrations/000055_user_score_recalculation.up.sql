ALTER TABLE `user_weight_scores`
  ADD COLUMN `calculation_version` varchar(32) NOT NULL DEFAULT 'legacy' AFTER `score`,
  ADD KEY `idx_user_weight_scores_calculation_version` (`calculation_version`, `source`, `id`);

UPDATE `user_weight_scores`
SET `calculation_version` = 'v1'
WHERE `source` = 'assessment';

CREATE TABLE `user_score_recalculation_jobs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `calculation_version` varchar(32) NOT NULL,
  `status` varchar(20) NOT NULL,
  `max_score_id` bigint unsigned NOT NULL,
  `cursor_score_id` bigint unsigned NOT NULL DEFAULT 0,
  `total_sessions` int unsigned NOT NULL DEFAULT 0,
  `processed_sessions` int unsigned NOT NULL DEFAULT 0,
  `failed_sessions` int unsigned NOT NULL DEFAULT 0,
  `estimated_cost_usd` decimal(12,6) NOT NULL DEFAULT 0,
  `last_error` text,
  `batch_lease_token` char(36) DEFAULT NULL,
  `batch_lease_until` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_score_recalculation_jobs_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
