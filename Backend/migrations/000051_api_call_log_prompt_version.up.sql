-- AI呼び出しごとに工程別プロンプト版とA/B群を記録する（#1778）。
ALTER TABLE `api_call_logs`
  ADD COLUMN `prompt_stage` varchar(64) NOT NULL DEFAULT 'unknown',
  ADD COLUMN `prompt_version` varchar(64) NOT NULL DEFAULT '',
  ADD COLUMN `prompt_variant` varchar(16) NOT NULL DEFAULT '';
