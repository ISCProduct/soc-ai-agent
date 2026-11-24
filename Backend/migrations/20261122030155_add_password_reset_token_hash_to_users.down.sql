ALTER TABLE `users`
  DROP KEY `uk_users_password_reset_token_hash`;

ALTER TABLE `users`
  DROP COLUMN `password_reset_token_hash`;
