-- 000004_add_organizations.down.sql

ALTER TABLE `resume_documents`
  DROP INDEX `idx_resume_documents_organization_id`,
  DROP COLUMN `organization_id`;

ALTER TABLE `interview_videos`
  DROP INDEX `idx_interview_videos_organization_id`,
  DROP COLUMN `organization_id`;

ALTER TABLE `interview_sessions`
  DROP INDEX `idx_interview_sessions_organization_id`,
  DROP COLUMN `organization_id`;

ALTER TABLE `user_weight_scores`
  DROP INDEX `idx_user_weight_scores_organization_id`,
  DROP COLUMN `organization_id`;

ALTER TABLE `chat_messages`
  DROP INDEX `idx_chat_messages_organization_id`,
  DROP COLUMN `organization_id`;

ALTER TABLE `users`
  DROP FOREIGN KEY `fk_users_organization`,
  DROP INDEX `idx_users_organization_id`,
  DROP COLUMN `organization_id`;

DROP TABLE IF EXISTS `organization_memberships`;
DROP TABLE IF EXISTS `organizations`;
