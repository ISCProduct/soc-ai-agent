-- 面接動画の保持期間を学校ごとに選ぶ（SOCAIAGENT-671）。
-- NULL は期限なし。既存の学校は既定の 90 日。
-- 公開（このマイグレーションの適用）から 30 日は、すでに残っている録画を消さない。
ALTER TABLE `schools`
  ADD COLUMN `interview_video_retention_days` int NULL DEFAULT 90;

CREATE TABLE `interview_video_retention_grace` (
  `id` tinyint unsigned NOT NULL,
  `grace_until` datetime(3) NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO `interview_video_retention_grace` (`id`, `grace_until`)
VALUES (1, DATE_ADD(NOW(3), INTERVAL 30 DAY));
