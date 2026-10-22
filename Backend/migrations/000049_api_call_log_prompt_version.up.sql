-- AI呼び出しごとに工程別プロンプト版とA/B群を記録する（#1778）。
ALTER TABLE `api_call_logs`
  ADD COLUMN `prompt_stage` varchar(64) NOT NULL DEFAULT '',
  ADD COLUMN `prompt_version` varchar(64) NOT NULL DEFAULT '',
  ADD COLUMN `prompt_variant` varchar(16) NOT NULL DEFAULT '',
  ADD KEY `idx_api_call_logs_prompt_stage_version_called_at` (`prompt_stage`, `prompt_version`, `called_at`),
  ALGORITHM=INPLACE, LOCK=NONE;
