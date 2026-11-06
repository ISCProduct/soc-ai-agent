ALTER TABLE `user_weight_scores`
  ADD COLUMN `calculation_version` varchar(32) NOT NULL DEFAULT 'legacy' AFTER `score`,
  ADD KEY `idx_user_weight_scores_calculation_version` (`calculation_version`, `source`, `id`);
