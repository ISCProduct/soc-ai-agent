-- #1294 のロールバック（TTSの文字数は失われる）
ALTER TABLE `api_call_logs`
  DROP COLUMN `characters`;
