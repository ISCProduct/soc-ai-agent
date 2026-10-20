ALTER TABLE `realtime_usage_logs`
  DROP KEY `idx_realtime_usage_logs_school_started_at`,
  DROP KEY `idx_realtime_usage_logs_user_started_at`,
  DROP COLUMN `school_id`,
  ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE `api_call_logs`
  DROP KEY `idx_api_call_logs_school_called_at`,
  DROP KEY `idx_api_call_logs_user_called_at`,
  DROP COLUMN `school_id`,
  ALGORITHM=INPLACE, LOCK=NONE;
