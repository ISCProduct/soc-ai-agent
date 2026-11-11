-- 列の追加とインデックスの追加を分ける。同じ文にするとテーブルの再構築になる。
-- 集計の SUM がインデックスだけで済むよう cost_usd まで含める。1校が表の大半を占める
-- 導入初期でも、学校の集計がフルスキャンにならない（#1776 の EXPLAIN で確認）。
ALTER TABLE `api_call_logs`
  ADD COLUMN `school_id` bigint unsigned NULL
    COMMENT 'AI呼び出し時点の所属校。NULLは所属校なし、または旧ログ',
  ALGORITHM=INSTANT;

ALTER TABLE `api_call_logs`
  ADD KEY `idx_api_call_logs_user_called_at` (`user_id`, `called_at`, `cost_usd`),
  ADD KEY `idx_api_call_logs_school_called_at` (`school_id`, `called_at`, `cost_usd`),
  ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE `realtime_usage_logs`
  ADD COLUMN `school_id` bigint unsigned NULL
    COMMENT 'Realtime開始時点の所属校。NULLは所属校なし、または旧ログ',
  ALGORITHM=INSTANT;

ALTER TABLE `realtime_usage_logs`
  ADD KEY `idx_realtime_usage_logs_user_started_at` (`user_id`, `started_at`),
  ADD KEY `idx_realtime_usage_logs_school_started_at` (`school_id`, `started_at`),
  ALGORITHM=INPLACE, LOCK=NONE;
