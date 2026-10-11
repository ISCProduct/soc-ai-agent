ALTER TABLE `session_validations`
  DROP KEY `idx_session_validations_user_id`,
  DROP COLUMN `user_id`;
