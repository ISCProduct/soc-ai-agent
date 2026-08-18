-- 000011_add_organization_plan_contract.up.sql
-- 学園(組織)の簡易契約管理: 料金プランと契約期間を追加

ALTER TABLE `organizations`
  ADD COLUMN `plan` varchar(20) NOT NULL DEFAULT 'free' AFTER `status`,
  ADD COLUMN `contract_start_date` date DEFAULT NULL AFTER `plan`,
  ADD COLUMN `contract_end_date` date DEFAULT NULL AFTER `contract_start_date`,
  ADD KEY `idx_organizations_plan` (`plan`);
