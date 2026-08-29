ALTER TABLE `users`
  ADD COLUMN `allow_scout_visibility` TINYINT(1) NOT NULL DEFAULT 0
    COMMENT '企業スカウト向け分析データの公開同意'
    AFTER `allow_collective_insight`;
