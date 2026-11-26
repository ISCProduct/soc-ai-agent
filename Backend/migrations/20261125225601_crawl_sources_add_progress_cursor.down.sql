-- SOCAIAGENT-726 の定期取得の進み具合の列を消す。
ALTER TABLE `crawl_sources` DROP COLUMN `progress_cursor`;
