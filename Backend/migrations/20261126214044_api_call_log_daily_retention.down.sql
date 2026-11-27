-- 注意: すでに api_call_logs の古い行を api_call_log_daily へ畳んで削除している場合、
-- このテーブルを DROP すると畳んだ合計（学校の利用額の根拠）が失われ、元の行からは復元できない。
-- 本番で畳み込みが走った後にロールバックする場合は、先に api_call_log_daily をバックアップすること。
DROP TABLE IF EXISTS `api_call_log_daily`;
