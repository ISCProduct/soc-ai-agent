ALTER TABLE `interview_reports`
  ADD COLUMN `evidence_utterance_ids_json` JSON DEFAULT NULL
  COMMENT '評点根拠に対応する受験者発話ID';
