ALTER TABLE user_company_matches
  DROP INDEX idx_ucm_user_favorited_created,
  ALGORITHM=INPLACE, LOCK=NONE;
