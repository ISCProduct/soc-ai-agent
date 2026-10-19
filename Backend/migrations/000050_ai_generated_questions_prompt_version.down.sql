ALTER TABLE `ai_generated_questions`
  DROP COLUMN `prompt_version`,
  ALGORITHM=INPLACE, LOCK=NONE;
