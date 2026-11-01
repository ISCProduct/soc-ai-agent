-- 追加した列とインデックスだけ戻す。
--
-- 受検データ（assessment_sessions / answers / results）と学校独自テンプレートは消さない。
-- school_id は 000048 で作られた列でこのロールバックでは落ちないため、戻した後も
-- どの学校の独自テンプレートかは判別できる。ここで DELETE すると、列を戻すだけの
-- ロールバックで学生の回答と結果まで失う（外部キーの CASCADE で連鎖する）。
ALTER TABLE `assessment_templates`
  DROP KEY `idx_assessment_templates_school_due`,
  DROP COLUMN `created_by`,
  DROP COLUMN `due_at`,
  DROP COLUMN `organization_id`;
