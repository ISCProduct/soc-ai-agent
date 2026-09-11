-- 000003_add_withdrawn_users.down.sql

DROP TABLE IF EXISTS `withdrawn_users`;

ALTER TABLE `users`
  DROP INDEX `idx_users_withdrawn_at`,
  DROP COLUMN `withdrawn_at`;
