-- #1166 のロールバック。
-- アプリを旧リビジョンへ戻す手順と同時に実行すること（000022 の down コメント参照）。
ALTER TABLE `user_company_matches` DROP INDEX `uniq_user_session_company`;
