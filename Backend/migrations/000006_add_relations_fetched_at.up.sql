ALTER TABLE `companies`
  ADD COLUMN `relations_fetched_at` datetime(3) DEFAULT NULL AFTER `tech_fetched_at`;
