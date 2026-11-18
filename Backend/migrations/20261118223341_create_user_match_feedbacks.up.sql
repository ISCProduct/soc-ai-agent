-- 推薦への「合う／合わない」のフィードバック（SOCAIAGENT-670）。
-- 推薦の精度（「参考」の判定・職種の加点が当たっているか）を、実際の学生の判断で測るために、判定だけを記録する。
-- 自由記述は持たない（個人情報を増やさない）。本人×推薦で 1 行で、押し直すと上書きする。
-- 計算が変わっても評価できるよう、押した時点の適合度・ずれる軸・職種の加点を写しで持つ。
CREATE TABLE `user_match_feedbacks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `match_id` bigint unsigned NOT NULL,
  -- fit（合う）/ unfit（合わない）
  `verdict` varchar(10) NOT NULL,
  `match_score` double NOT NULL DEFAULT 0,
  `fit_mismatch_axes` varchar(120) NOT NULL DEFAULT '',
  `job_matched` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_user_match_feedback` (`user_id`, `match_id`),
  KEY `idx_user_match_feedbacks_match` (`match_id`),
  -- 退会のパージ（hardDeleteUserData）では明示的に消す。CASCADE は消し忘れの保険。
  CONSTRAINT `fk_user_match_feedbacks_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_user_match_feedbacks_match` FOREIGN KEY (`match_id`) REFERENCES `user_company_matches` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
