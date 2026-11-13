-- 再計算で置き換える前のスコアを残す。LLM を使う再計算は非決定的なので、
-- 影響を確認してから元に戻せるようにする（#1770）。
CREATE TABLE `user_score_recalculation_snapshots` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `job_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `session_id` varchar(100) NOT NULL,
  `scores_json` json NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_user_score_recalculation_snapshots_session` (`job_id`, `user_id`, `session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
