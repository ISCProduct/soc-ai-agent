-- #1166 のロールバック（集約した重複行は復元しない）
ALTER TABLE `user_company_matches` DROP INDEX `uniq_user_session_company`;
