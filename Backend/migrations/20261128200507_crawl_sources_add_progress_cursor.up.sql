-- 定期取得の進み具合（EDINET の書類一覧を「どの日まで処理したか」など）。種類ごとに中身の形式は異なる。
-- 本番は稼働日しか起動しないため、最後の実行日時（last_run_at）ではなく処理済みの位置で再開する。
ALTER TABLE `crawl_sources`
  ADD COLUMN `progress_cursor` varchar(64) NOT NULL DEFAULT '' COMMENT '定期取得の処理済み位置';
