-- 採点モデルの学習データの書き出し（#2168）の監査ログ。書き出し1回につき1行。
-- 個人を特定するもの（利用者 ID など）は持たず、件数と出力先だけを残す。
-- そのため退会時のパージ（hardDeleteUserData）の対象にしない。
CREATE TABLE `training_scorer_export_runs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `status` varchar(16) NOT NULL COMMENT 'running / succeeded / failed',
  `started_at` datetime(3) NOT NULL,
  `finished_at` datetime(3) DEFAULT NULL,
  `student_count` int NOT NULL DEFAULT 0,
  `session_count` int NOT NULL DEFAULT 0,
  `destination` varchar(1024) NOT NULL COMMENT '出力先 s3://bucket/prefix/runs/<実行ID>/',
  `error_message` text,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_training_scorer_export_runs_started_at` (`started_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
