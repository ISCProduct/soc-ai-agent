ALTER TABLE `user_application_statuses`
  DROP KEY `uniq_user_application_statuses_active`,
  DROP COLUMN `active_dedup_key`;
