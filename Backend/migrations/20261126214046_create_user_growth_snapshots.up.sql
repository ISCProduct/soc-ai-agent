-- 10カテゴリのスコアと診断フェーズの進みを、変わった時点の写しで残す（SOCAIAGENT-601）。
-- user_weight_scores と user_analysis_progress は最新だけなので、過去との差はここが無いと出せない。
-- 再計算の控え（user_score_recalculation_snapshots）とは別。再計算は成長ではないので、この表には入れない。
-- この機能を入れる前の値は復元できない。線は記録が始まった日からのみ。
CREATE TABLE `user_growth_snapshots` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  -- chat / interview / resume / es / assessment
  `source` varchar(32) NOT NULL,
  `session_id` varchar(100) NOT NULL,
  -- その時点のセッションにあるカテゴリだけ。無いカテゴリを50で埋めない（マッチングの中立値と混ぜない）。
  `scores_json` json NOT NULL,
  -- 診断フェーズの進み。適性検査のセッションには進捗が無いので、記録時の診断フェーズを写すことがある。
  `phase_progress_json` json NOT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_user_growth_snapshots_user_time` (`user_id`, `created_at`),
  -- 退会のパージ（hardDeleteUserData）では明示的に消す。CASCADE は消し忘れの保険。
  CONSTRAINT `fk_user_growth_snapshots_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
