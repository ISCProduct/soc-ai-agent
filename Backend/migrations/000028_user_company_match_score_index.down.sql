ALTER TABLE user_company_matches
  DROP INDEX idx_ucm_user_session_score,
  ALGORITHM=INPLACE, LOCK=NONE;
