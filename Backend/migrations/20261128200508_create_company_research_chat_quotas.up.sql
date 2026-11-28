-- 企業研究AIチャットの1日の質問回数（SOCAIAGENT-724）。利用者×日付で1行。
-- api_call_logs の行数で数えると、1問でベクトル化と回答の2行が付いて上限が早く来るうえ、
-- 確認と記録が別操作なので同時に質問すると上限を超えられる（レビュー指摘）。
-- 質問のたびにこの行を行ロックして1つ増やし、上限に達していたら増やさずに断る。
CREATE TABLE `company_research_chat_quotas` (
  `user_id` bigint unsigned NOT NULL,
  -- 日本時間の日付
  `quota_date` date NOT NULL,
  `used` int unsigned NOT NULL DEFAULT 0,
  `updated_at` datetime(3) NULL,
  PRIMARY KEY (`user_id`, `quota_date`),
  -- 退会のパージ（hardDeleteUserData）では明示的に消す。CASCADE は消し忘れの保険。
  CONSTRAINT `fk_company_research_chat_quotas_user` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
