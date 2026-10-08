ALTER TABLE `session_validations`
  ADD COLUMN `user_id` BIGINT UNSIGNED NULL AFTER `session_id`,
  ADD KEY `idx_session_validations_user_id` (`user_id`);
