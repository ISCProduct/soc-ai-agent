-- 平文のトークンは戻さない。列だけを戻す。
ALTER TABLE `company_entry_submissions`
  ADD COLUMN `invite_token` varchar(255) DEFAULT NULL;
