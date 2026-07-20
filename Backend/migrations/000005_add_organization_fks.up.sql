-- 000005_add_organization_fks.up.sql
-- 主要ユーザーデータテーブルの organization_id に FK を付与する (#611 review)

ALTER TABLE `chat_messages`
  ADD CONSTRAINT `fk_chat_messages_organization`
  FOREIGN KEY (`organization_id`) REFERENCES `organizations` (`id`);

ALTER TABLE `user_weight_scores`
  ADD CONSTRAINT `fk_user_weight_scores_organization`
  FOREIGN KEY (`organization_id`) REFERENCES `organizations` (`id`);

ALTER TABLE `interview_sessions`
  ADD CONSTRAINT `fk_interview_sessions_organization`
  FOREIGN KEY (`organization_id`) REFERENCES `organizations` (`id`);

ALTER TABLE `interview_videos`
  ADD CONSTRAINT `fk_interview_videos_organization`
  FOREIGN KEY (`organization_id`) REFERENCES `organizations` (`id`);

ALTER TABLE `resume_documents`
  ADD CONSTRAINT `fk_resume_documents_organization`
  FOREIGN KEY (`organization_id`) REFERENCES `organizations` (`id`);
