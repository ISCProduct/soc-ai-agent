-- 軽量採点モデルのシャドー結果（SOCAIAGENT-622）。
-- LLM の採点（interview_reports.scores_json）や user_weight_scores、マッチングには使わない。
CREATE TABLE `interview_shadow_scores` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `session_id` bigint unsigned NOT NULL,
  `model_version` varchar(64) NOT NULL COMMENT 'モデルファイル全体の SHA-256（重みだけのハッシュではない）',
  `scores_json` json NOT NULL COMMENT '軸ごとのスコアと確信度',
  `latency_ms` int NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_interview_shadow_scores_session` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
