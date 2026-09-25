-- #1294 のロールバック（機能別・組織別の内訳と音声秒数・レイテンシは失われる）
--
-- 既存列（provider / via_fallback / cost_usd / トークン）には触れない。
-- 戻しても #1293 のフォールバック予算判定はそのまま動く。
ALTER TABLE `api_call_logs`
  DROP KEY `idx_api_call_logs_org_called_at`,
  DROP KEY `idx_api_call_logs_called_at_provider`,
  DROP COLUMN `cache_hit`,
  DROP COLUMN `latency_ms`,
  DROP COLUMN `audio_seconds`,
  DROP COLUMN `organization_id`,
  DROP COLUMN `user_id`,
  DROP COLUMN `feature`;
