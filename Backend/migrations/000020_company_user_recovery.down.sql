-- #1196 のロールバック

ALTER TABLE `company_student_tags`
  DROP FOREIGN KEY `fk_company_student_tags_creator`;
ALTER TABLE `company_student_tags`
  ADD CONSTRAINT `fk_company_student_tags_creator`
  FOREIGN KEY (`created_by`) REFERENCES `company_users` (`id`) ON DELETE CASCADE;

-- 000017 の invite_token には一意キーが無かったので、列だけを戻す。
ALTER TABLE `company_users`
  ADD COLUMN `invite_token` varchar(255) DEFAULT NULL;

ALTER TABLE `company_users`
  DROP KEY `uk_company_users_password_reset_hash`,
  DROP KEY `uk_company_users_invite_token_hash`;

ALTER TABLE `company_users`
  DROP COLUMN `password_reset_token_hash`,
  DROP COLUMN `password_reset_expires_at`,
  DROP COLUMN `disabled_at`,
  DROP COLUMN `invite_token_hash`;
