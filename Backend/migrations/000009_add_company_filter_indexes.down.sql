ALTER TABLE `company_relations`
  DROP INDEX `idx_company_relations_active_to`,
  DROP INDEX `idx_company_relations_active_from`,
  DROP INDEX `idx_company_relations_active_child`,
  DROP INDEX `idx_company_relations_active_parent`;

ALTER TABLE `company_job_positions`
  DROP INDEX `idx_company_job_positions_active_status`;

ALTER TABLE `companies`
  DROP INDEX `idx_companies_active_status_id`,
  DROP INDEX `idx_companies_active_status_industry`;
