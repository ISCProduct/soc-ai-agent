ALTER TABLE `api_call_logs`
  DROP KEY `idx_api_call_logs_prompt_stage_version_called_at`,
  DROP COLUMN `prompt_variant`,
  DROP COLUMN `prompt_version`,
  DROP COLUMN `prompt_stage`,
  ALGORITHM=INPLACE, LOCK=NONE;
