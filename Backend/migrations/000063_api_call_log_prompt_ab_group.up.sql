-- プロンプトの A/B で割り当てた版を記録する（#1976）。A/B の対象外の呼び出しは空。
ALTER TABLE `api_call_logs`
  ADD COLUMN `prompt_ab_group` varchar(64) NOT NULL DEFAULT '';
