-- 000005_add_organization_fks.down.sql

ALTER TABLE `resume_documents` DROP FOREIGN KEY `fk_resume_documents_organization`;
ALTER TABLE `interview_videos` DROP FOREIGN KEY `fk_interview_videos_organization`;
ALTER TABLE `interview_sessions` DROP FOREIGN KEY `fk_interview_sessions_organization`;
ALTER TABLE `user_weight_scores` DROP FOREIGN KEY `fk_user_weight_scores_organization`;
ALTER TABLE `chat_messages` DROP FOREIGN KEY `fk_chat_messages_organization`;
