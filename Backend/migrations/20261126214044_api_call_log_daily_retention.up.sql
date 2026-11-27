-- api_call_logs は呼び出しのたびに増えるだけで、消す処理が無かった（SOCAIAGENT-528）。
-- 24ヶ月より古い行は、単価の再計算に必要な合計へ畳んでから消す。
-- 利用者の ID と遅延は畳まない。退会の削除では消さない（学校の利用額に穴が開くため）。
-- school_id / organization_id の 0 は、元の行が NULL（未所属・未配賦）だったことを表す。
CREATE TABLE `api_call_log_daily` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `called_on` date NOT NULL,
  `model` varchar(100) NOT NULL,
  `feature` varchar(64) NOT NULL,
  `provider` varchar(32) NOT NULL,
  `school_id` bigint unsigned NOT NULL DEFAULT 0,
  `organization_id` bigint unsigned NOT NULL DEFAULT 0,
  `prompt_stage` varchar(64) NOT NULL,
  `prompt_version` varchar(64) NOT NULL,
  `prompt_ab_group` varchar(64) NOT NULL,
  `price_basis` varchar(16) NOT NULL,
  `via_fallback` tinyint(1) NOT NULL,
  `prompt_tokens` bigint NOT NULL DEFAULT 0,
  `completion_tokens` bigint NOT NULL DEFAULT 0,
  `total_tokens` bigint NOT NULL DEFAULT 0,
  `cost_usd` double NOT NULL DEFAULT 0,
  `audio_seconds` decimal(18,2) NOT NULL DEFAULT 0,
  `characters` bigint NOT NULL DEFAULT 0,
  `web_search_calls` bigint NOT NULL DEFAULT 0,
  `call_count` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_api_call_log_daily` (
    `called_on`, `model`, `feature`, `provider`, `school_id`, `organization_id`,
    `prompt_stage`, `prompt_version`, `prompt_ab_group`, `price_basis`, `via_fallback`
  )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
