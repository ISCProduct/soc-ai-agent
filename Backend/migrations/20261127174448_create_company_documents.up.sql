-- 企業研究AIの材料のうち、文章（SOCAIAGENT-722 / 725）。
-- 公的資料・公式サイトから取得した文章をそのまま持つ。RAG(Chroma)のベクトルはここから作り直せるようにする
-- （Chroma が消えても取り直さずに済む）。content_hash が同じなら作り直さない。
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
  -- RAG(Chroma)に登録した日時。本文が変わったら NULL に戻し、登録ジョブが拾い直す
  `indexed_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_company_documents_indexed_at` (`indexed_at`),
  UNIQUE KEY `uniq_company_documents_source` (`company_id`, `source_type`, `source_ref`),
  CONSTRAINT `fk_company_documents_company` FOREIGN KEY (`company_id`) REFERENCES `companies` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
