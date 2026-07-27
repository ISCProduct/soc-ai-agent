-- 000004_add_organizations.up.sql
-- マルチテナント基盤 (#611)
-- organizations / organization_memberships を新設し、主要ユーザーデータに organization_id を付与する

CREATE TABLE `organizations` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `slug` varchar(100) NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_organizations_slug` (`slug`),
  KEY `idx_organizations_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `organization_memberships` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `organization_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `role` varchar(20) NOT NULL DEFAULT 'member',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_org_membership_user_org` (`organization_id`,`user_id`),
  UNIQUE KEY `idx_org_membership_user` (`user_id`),
  KEY `idx_org_memberships_role` (`role`),
  CONSTRAINT `fk_org_memberships_organization` FOREIGN KEY (`organization_id`) REFERENCES `organizations` (`id`),
  CONSTRAINT `fk_org_memberships_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 既存データを収容するデフォルト組織
INSERT INTO `organizations` (`id`, `name`, `slug`, `status`, `created_at`, `updated_at`)
VALUES (1, 'Default Organization', 'default', 'active', NOW(3), NOW(3));

ALTER TABLE `users`
  ADD COLUMN `organization_id` bigint unsigned DEFAULT NULL AFTER `id`,
  ADD KEY `idx_users_organization_id` (`organization_id`);

UPDATE `users` SET `organization_id` = 1 WHERE `organization_id` IS NULL;

INSERT INTO `organization_memberships` (`organization_id`, `user_id`, `role`, `created_at`, `updated_at`)
SELECT 1, `id`,
  CASE WHEN `is_admin` = 1 THEN 'owner' ELSE 'member' END,
  NOW(3), NOW(3)
FROM `users`;

ALTER TABLE `users`
  MODIFY COLUMN `organization_id` bigint unsigned NOT NULL,
  ADD CONSTRAINT `fk_users_organization` FOREIGN KEY (`organization_id`) REFERENCES `organizations` (`id`);

-- 主要ユーザーデータの組織スコープ列
ALTER TABLE `chat_messages`
  ADD COLUMN `organization_id` bigint unsigned DEFAULT NULL AFTER `id`,
  ADD KEY `idx_chat_messages_organization_id` (`organization_id`);

ALTER TABLE `user_weight_scores`
  ADD COLUMN `organization_id` bigint unsigned DEFAULT NULL AFTER `id`,
  ADD KEY `idx_user_weight_scores_organization_id` (`organization_id`);

ALTER TABLE `interview_sessions`
  ADD COLUMN `organization_id` bigint unsigned DEFAULT NULL AFTER `id`,
  ADD KEY `idx_interview_sessions_organization_id` (`organization_id`);

ALTER TABLE `interview_videos`
  ADD COLUMN `organization_id` bigint unsigned DEFAULT NULL AFTER `id`,
  ADD KEY `idx_interview_videos_organization_id` (`organization_id`);

ALTER TABLE `resume_documents`
  ADD COLUMN `organization_id` bigint unsigned DEFAULT NULL AFTER `id`,
  ADD KEY `idx_resume_documents_organization_id` (`organization_id`);

UPDATE `chat_messages` cm
  INNER JOIN `users` u ON u.id = cm.user_id
  SET cm.organization_id = u.organization_id
  WHERE cm.organization_id IS NULL;

UPDATE `user_weight_scores` s
  INNER JOIN `users` u ON u.id = s.user_id
  SET s.organization_id = u.organization_id
  WHERE s.organization_id IS NULL;

UPDATE `interview_sessions` s
  INNER JOIN `users` u ON u.id = s.user_id
  SET s.organization_id = u.organization_id
  WHERE s.organization_id IS NULL;

UPDATE `interview_videos` v
  INNER JOIN `users` u ON u.id = v.user_id
  SET v.organization_id = u.organization_id
  WHERE v.organization_id IS NULL;

UPDATE `resume_documents` d
  INNER JOIN `users` u ON u.id = d.user_id
  SET d.organization_id = u.organization_id
  WHERE d.organization_id IS NULL;

-- 孤児行（ユーザー削除済み等）はデフォルト組織へ
UPDATE `chat_messages` SET `organization_id` = 1 WHERE `organization_id` IS NULL;
UPDATE `user_weight_scores` SET `organization_id` = 1 WHERE `organization_id` IS NULL;
UPDATE `interview_sessions` SET `organization_id` = 1 WHERE `organization_id` IS NULL;
UPDATE `interview_videos` SET `organization_id` = 1 WHERE `organization_id` IS NULL;
UPDATE `resume_documents` SET `organization_id` = 1 WHERE `organization_id` IS NULL;

ALTER TABLE `chat_messages`
  MODIFY COLUMN `organization_id` bigint unsigned NOT NULL;

ALTER TABLE `user_weight_scores`
  MODIFY COLUMN `organization_id` bigint unsigned NOT NULL;

ALTER TABLE `interview_sessions`
  MODIFY COLUMN `organization_id` bigint unsigned NOT NULL;

ALTER TABLE `interview_videos`
  MODIFY COLUMN `organization_id` bigint unsigned NOT NULL;

ALTER TABLE `resume_documents`
  MODIFY COLUMN `organization_id` bigint unsigned NOT NULL;
