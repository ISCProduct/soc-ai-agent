ALTER TABLE `pending_registrations`
  DROP KEY `idx_pending_reg_submission_id`,
  DROP KEY `idx_pending_reg_company_id`,
  DROP COLUMN `submission_id`,
  DROP COLUMN `company_id`;

DROP TABLE IF EXISTS `company_ownerships`;
DROP TABLE IF EXISTS `company_entry_submissions`;
