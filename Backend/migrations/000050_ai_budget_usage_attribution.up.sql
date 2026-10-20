ALTER TABLE `api_call_logs`
  ADD COLUMN `school_id` bigint unsigned NULL
    COMMENT 'AI呼び出し時点の所属校。NULLは所属校なし、または旧ログ',
  ADD KEY `idx_api_call_logs_user_called_at` (`user_id`, `called_at`),
  ADD KEY `idx_api_call_logs_school_called_at` (`school_id`, `called_at`),
  ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE `realtime_usage_logs`
  ADD COLUMN `school_id` bigint unsigned NULL
    COMMENT 'Realtime開始時点の所属校。NULLは所属校なし、または旧ログ',
  ADD KEY `idx_realtime_usage_logs_user_started_at` (`user_id`, `started_at`),
  ADD KEY `idx_realtime_usage_logs_school_started_at` (`school_id`, `started_at`),
  ALGORITHM=INPLACE, LOCK=NONE;
