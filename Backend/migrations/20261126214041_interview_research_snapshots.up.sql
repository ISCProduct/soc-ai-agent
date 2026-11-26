-- Web 調査で得た面接の質問候補を、企業と職種ごとに期限つきで持つ（SOCAIAGENT-651）。
-- 面接の途中では検索せず、期限内の行だけを使う。
CREATE TABLE interview_research_snapshots (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  company_key VARCHAR(191) NOT NULL,
  position VARCHAR(191) NOT NULL DEFAULT '',
  questions_json TEXT NOT NULL,
  fetched_at DATETIME(3) NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_research_company_position (company_key, position),
  KEY idx_interview_research_snapshots_expires_at (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
