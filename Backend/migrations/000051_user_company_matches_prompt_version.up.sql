-- マッチング理由に使った工程別プロンプト版を保存する（#1778）。
ALTER TABLE `user_company_matches`
  ADD COLUMN `match_reason_prompt_version` varchar(64) NOT NULL DEFAULT '',
  ALGORITHM=INPLACE, LOCK=NONE;
