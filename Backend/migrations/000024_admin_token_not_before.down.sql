-- #1155 のロールバック（失効指定は失われる）
ALTER TABLE `users` DROP COLUMN `admin_token_not_before`;
