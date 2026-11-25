-- SOCAIAGENT-722 / 724 の企業研究AIの材料テーブルを消す。
DROP TABLE IF EXISTS `company_facts`;
DROP TABLE IF EXISTS `company_documents`;
ALTER TABLE `crawl_sources` DROP COLUMN `progress_cursor`;
