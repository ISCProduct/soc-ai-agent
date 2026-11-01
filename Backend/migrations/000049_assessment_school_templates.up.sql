-- 適性検査 第2段: 学校独自テンプレートと先生向け受検状況（#1675）
--
-- school_id が非NULLのテンプレートは学校独自。is_standard=0 固定で、マッチングには合流させない
-- （A校の80点とB校の80点は別物のため）。
-- organization_id: 学校独自テンプレートのテナント境界。先生の一覧・作成は所属学校で明示的に絞る。
-- due_at: 受検期限。先生向け一覧の「期限超過」判定に使う。NULLは期限なし。
-- created_by: 作成した職員の users.id。監査用で、外部キーは張らない（退会・削除で検査を巻き込まない）。

ALTER TABLE `assessment_templates`
  ADD COLUMN `organization_id` bigint unsigned DEFAULT NULL AFTER `school_id`,
  ADD COLUMN `due_at` datetime(3) DEFAULT NULL AFTER `is_standard`,
  ADD COLUMN `created_by` bigint unsigned DEFAULT NULL AFTER `due_at`,
  ADD KEY `idx_assessment_templates_school_due` (`school_id`, `due_at`);
