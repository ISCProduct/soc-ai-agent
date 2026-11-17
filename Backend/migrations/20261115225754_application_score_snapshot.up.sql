-- 応募時点のスコアスナップショット（#203 フライホイール）
-- マッチングに使った session のカテゴリ別スコアを JSON で残し、
-- キャリブレーションが「応募後に動いた最新スコア」ではなく応募時点を使えるようにする。
ALTER TABLE `user_application_statuses`
  ADD COLUMN `score_snapshot` TEXT NULL COMMENT '応募時点の user_weight_scores JSON' AFTER `notes`;
