ALTER TABLE `user_weight_scores`
  DROP INDEX `idx_user_weight_scores_calculation_version`,
  DROP COLUMN `calculation_version`;
