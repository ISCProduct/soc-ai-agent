ALTER TABLE `graduate_employments`
  DROP FOREIGN KEY `fk_graduate_employments_school`,
  DROP KEY `idx_graduate_employments_school_id`,
  DROP COLUMN `school_id`;

ALTER TABLE `users`
  DROP FOREIGN KEY `fk_users_school`,
  DROP KEY `idx_users_school_id`,
  DROP COLUMN `school_id`;

DROP TABLE IF EXISTS `school_company_approvals`;
DROP TABLE IF EXISTS `admin_school_memberships`;
DROP TABLE IF EXISTS `schools`;
