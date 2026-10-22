ALTER TABLE `user_company_matches`
  DROP COLUMN `match_reason_prompt_version`,
  ALGORITHM=INPLACE, LOCK=NONE;
