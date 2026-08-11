-- 個別校(情報科学専門学校/横浜デジタルアーツ専門学校 等)を構造化データとして持つ
-- Organization(学園)配下の個別校単位で管理メニューを絞り込むための基盤

CREATE TABLE `schools` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `organization_id` bigint unsigned NOT NULL,
  `name` varchar(255) NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_schools_org_name` (`organization_id`,`name`),
  KEY `idx_schools_status` (`status`),
  CONSTRAINT `fk_schools_organization` FOREIGN KEY (`organization_id`) REFERENCES `organizations` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 管理者(先生)がどの学校を担当しているか。0件のadminは「システム管理者」= 無制限として扱う。
CREATE TABLE `admin_school_memberships` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `school_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_admin_school_memberships_user_school` (`user_id`,`school_id`),
  KEY `idx_admin_school_memberships_school_id` (`school_id`),
  CONSTRAINT `fk_admin_school_memberships_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`),
  CONSTRAINT `fk_admin_school_memberships_school` FOREIGN KEY (`school_id`) REFERENCES `schools` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 学校ごとに先生が承認した企業だけを、その学校配下の管理者に見せるための中間テーブル
CREATE TABLE `school_company_approvals` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `school_id` bigint unsigned NOT NULL,
  `company_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_school_company_approvals` (`school_id`,`company_id`),
  KEY `idx_school_company_approvals_company_id` (`company_id`),
  CONSTRAINT `fk_school_company_approvals_school` FOREIGN KEY (`school_id`) REFERENCES `schools` (`id`),
  CONSTRAINT `fk_school_company_approvals_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- users / graduate_employments の自由記述 school_name とは別に、構造化された school_id を持たせる。
-- 既存データは自動突合しない(NULLのまま)。以後の登録・プロフィール更新で完全一致した場合のみベストエフォートで設定する。
ALTER TABLE `users`
  ADD COLUMN `school_id` bigint unsigned DEFAULT NULL AFTER `organization_id`,
  ADD KEY `idx_users_school_id` (`school_id`),
  ADD CONSTRAINT `fk_users_school` FOREIGN KEY (`school_id`) REFERENCES `schools` (`id`);

ALTER TABLE `graduate_employments`
  ADD COLUMN `school_id` bigint unsigned DEFAULT NULL AFTER `school_name`,
  ADD KEY `idx_graduate_employments_school_id` (`school_id`),
  ADD CONSTRAINT `fk_graduate_employments_school` FOREIGN KEY (`school_id`) REFERENCES `schools` (`id`);
