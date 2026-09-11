ALTER TABLE `companies`
  ADD COLUMN `employee_count_basis` varchar(16) NOT NULL DEFAULT '' AFTER `employee_count`;
