-- AI生成質問に使った工程別プロンプト版を保存する（#1778）。
ALTER TABLE `ai_generated_questions`
  ADD COLUMN `prompt_version` varchar(64) NOT NULL DEFAULT '',
  ALGORITHM=INPLACE, LOCK=NONE;
