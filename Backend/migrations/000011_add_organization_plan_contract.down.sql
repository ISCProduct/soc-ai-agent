-- 000011_add_organization_plan_contract.down.sql

ALTER TABLE `organizations`
  DROP KEY `idx_organizations_plan`,
  DROP COLUMN `contract_end_date`,
  DROP COLUMN `contract_start_date`,
  DROP COLUMN `plan`;
