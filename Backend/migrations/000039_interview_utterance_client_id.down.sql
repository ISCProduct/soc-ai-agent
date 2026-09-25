ALTER TABLE `interview_utterances`
  DROP INDEX `uniq_interview_utterances_session_client`,
  DROP COLUMN `client_utterance_id`;
