-- 同一企業→同一学生のスカウト送信を直列化するためのロック行（#1095）
-- クールダウン判定と INSERT を同じトランザクションで行うとき、
-- 履歴がまだ無い組でも行ロックを取れるようにする。

CREATE TABLE IF NOT EXISTS `scout_send_locks` (
  `company_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  PRIMARY KEY (`company_id`, `user_id`),
  CONSTRAINT `fk_scout_send_locks_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_scout_send_locks_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
