DROP INDEX `idx_companies_guest_entry` ON `companies`;
ALTER TABLE `companies` DROP COLUMN `is_guest_entry`;
