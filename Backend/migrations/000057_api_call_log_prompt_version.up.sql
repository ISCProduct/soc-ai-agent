-- AI呼び出しごとに工程別プロンプト版を記録する（#1778）。
ALTER TABLE `api_call_logs`
  ADD COLUMN `prompt_stage` varchar(64) NOT NULL DEFAULT 'unknown',
  ADD COLUMN `prompt_version` varchar(64) NOT NULL DEFAULT '';
