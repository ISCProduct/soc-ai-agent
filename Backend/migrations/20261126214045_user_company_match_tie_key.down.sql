ALTER TABLE user_company_matches
  DROP INDEX idx_ucm_user_session_rank,
  DROP COLUMN match_tie_key,
  ADD INDEX idx_ucm_user_session_score (user_id, session_id, match_score DESC);
