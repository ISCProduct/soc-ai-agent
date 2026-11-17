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
  -- 実行中・一時停止中だけ 1、それ以外は NULL。一意制約は NULL を重複とみなさないので、
  -- 進行中のジョブを1件に制限できる（#1770。同時に開始しても LLM 費用が2重にならない）。
  `active_slot` tinyint GENERATED ALWAYS AS (CASE WHEN `status` IN ('running', 'paused') THEN 1 ELSE NULL END) VIRTUAL,
  PRIMARY KEY (`id`),
  KEY `idx_user_score_recalculation_jobs_status` (`status`),
  UNIQUE KEY `uq_user_score_recalculation_jobs_active_slot` (`active_slot`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
