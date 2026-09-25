-- #1293 のロールバック（推論先の内訳とフォールバック判定は失われる）
--
-- 戻した直後はフォールバックのUSD上限が「全コール合計」で判定されるため、
-- 通常の OpenAI 利用で上限に達しフォールバックが発動しなくなる点に注意。
ALTER TABLE `api_call_logs`
  DROP KEY `idx_api_call_logs_fallback_called_at`,
  DROP COLUMN `via_fallback`,
  DROP COLUMN `provider`;
