-- 学校ごとの個別求人停止（#1508）。
-- 企業を承認すると求人は自動で学生に出るが、問題のある求人だけを
-- その学校向けに個別停止できるようにする。承認は企業単位のまま。
CREATE TABLE `school_job_suppressions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `school_id` bigint unsigned NOT NULL,
  `job_position_id` bigint unsigned NOT NULL,
  `suppressed_by` bigint unsigned NOT NULL,
  `reason` text,
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sjs_school_job` (`school_id`, `job_position_id`),
  KEY `idx_sjs_job` (`job_position_id`),
  CONSTRAINT `fk_sjs_school` FOREIGN KEY (`school_id`) REFERENCES `schools` (`id`),
  CONSTRAINT `fk_sjs_job` FOREIGN KEY (`job_position_id`) REFERENCES `company_job_positions` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
