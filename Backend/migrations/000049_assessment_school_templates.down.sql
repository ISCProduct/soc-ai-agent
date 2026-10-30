-- 学校独自テンプレート（school_id が非NULL）とその受検データを先に消してから列を落とす。
-- 順序を逆にすると、所有者を判別できない孤児の学校テンプレートが残る。
-- assessment_answers / assessment_results は session への ON DELETE CASCADE、
-- assessment_questions は template への ON DELETE CASCADE で連鎖して消える。
-- user_weight_scores には書いていない（学校独自はマッチングに合流しない）ので触らない。
DELETE FROM `assessment_sessions`
 WHERE `template_id` IN (SELECT `id` FROM `assessment_templates` WHERE `school_id` IS NOT NULL);
DELETE FROM `assessment_templates` WHERE `school_id` IS NOT NULL;

ALTER TABLE `assessment_templates`
  DROP KEY `idx_assessment_templates_school_due`,
  DROP COLUMN `created_by`,
  DROP COLUMN `due_at`,
  DROP COLUMN `organization_id`;
