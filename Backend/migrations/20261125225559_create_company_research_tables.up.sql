-- 企業研究AIの材料（SOCAIAGENT-722 / 724）。
-- company_documents: 公的資料・公式サイトから取得した文章をそのまま持つ。RAG(Chroma)のベクトルは
--   ここから作り直せるようにする（Chroma が消えても取り直さずに済む）。content_hash が同じなら作り直さない。
-- company_facts: 数字を1項目ずつ、出典と対象年度つきで持つ。「どの年度の誰の数字か」を答えに添えるため。
--   同じ項目でも情報源ごとに1行（EDINET と gBizINFO で値が違えば両方残す）。
CREATE TABLE `company_documents` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `company_id` bigint unsigned NOT NULL,
  -- edinet / gbizinfo / official_site / web_search
  `source_type` varchar(32) NOT NULL,
  -- 情報源の中での識別子と区分（EDINET なら書類管理番号＋文章の種類、公式サイトなら URL のハッシュ）
  `source_ref` varchar(128) NOT NULL,
  `title` varchar(255) NOT NULL DEFAULT '',
  `url` varchar(500) NOT NULL DEFAULT '',
  -- 対象期間の末日（有価証券報告書の期末など）。不明なら NULL
  `as_of` date DEFAULT NULL,
  `fetched_at` datetime(3) NOT NULL,
  `content_hash` char(64) NOT NULL,
  `body` mediumtext NOT NULL,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_company_documents_source` (`company_id`, `source_type`, `source_ref`),
  CONSTRAINT `fk_company_documents_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `company_facts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `company_id` bigint unsigned NOT NULL,
  -- employees_consolidated / employees_single / average_age / average_service_years / average_salary / net_sales など
  `field` varchar(64) NOT NULL,
  `value` varchar(255) NOT NULL,
  -- 人 / 歳 / 年 / 円 など。値の解釈に必要なので必ず入れる
  `unit` varchar(16) NOT NULL DEFAULT '',
  `source_type` varchar(32) NOT NULL,
  `source_url` varchar(500) NOT NULL DEFAULT '',
  `as_of` date DEFAULT NULL,
  `fetched_at` datetime(3) NOT NULL,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_company_facts_field_source` (`company_id`, `field`, `source_type`),
  CONSTRAINT `fk_company_facts_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 定期取得の進み具合（EDINET の書類一覧を「どの日まで処理したか」など）。種類ごとに中身の形式は異なる。
-- 本番は稼働日しか起動しないため、最後の実行日時（last_run_at）ではなく処理済みの位置で再開する。
ALTER TABLE `crawl_sources`
  ADD COLUMN `progress_cursor` varchar(64) NOT NULL DEFAULT '' COMMENT '定期取得の処理済み位置';
