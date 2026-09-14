-- 診断妥当性レポート（スコアは自動補正しない。フラグと信頼度のみ）
CREATE TABLE IF NOT EXISTS diagnosis_quality_reports (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  session_id VARCHAR(255) NOT NULL,
  confidence INT NOT NULL DEFAULT 0 COMMENT '0-100',
  flags_json JSON NULL,
  summary TEXT NULL,
  raw_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uniq_diagnosis_quality_user_session (user_id, session_id),
  KEY idx_diagnosis_quality_user (user_id),
  KEY idx_diagnosis_quality_confidence (confidence)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
