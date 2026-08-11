-- companies / job_positions / relations のフィルタ・参照用インデックス (#748)
ALTER TABLE `companies`
  ADD INDEX `idx_companies_active_status_industry` (`is_active`, `data_status`, `industry`),
  ADD INDEX `idx_companies_active_status_id` (`is_active`, `data_status`, `id`);

ALTER TABLE `company_job_positions`
  ADD INDEX `idx_company_job_positions_active_status` (`is_active`, `data_status`);

ALTER TABLE `company_relations`
  ADD INDEX `idx_company_relations_active_parent` (`is_active`, `parent_id`),
  ADD INDEX `idx_company_relations_active_child` (`is_active`, `child_id`),
  ADD INDEX `idx_company_relations_active_from` (`is_active`, `from_id`),
  ADD INDEX `idx_company_relations_active_to` (`is_active`, `to_id`);
