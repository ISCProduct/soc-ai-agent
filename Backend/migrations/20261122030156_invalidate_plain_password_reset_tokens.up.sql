-- 平文で保存されている未使用の再設定トークンを無効化する（SOCAIAGENT-693）。
-- ハッシュから平文は戻せず、平文をハッシュ化して移すと平文が DB に残る期間ができるため、移さない。
-- 再設定リンクは短命なので、利用者は再送で復旧できる。
UPDATE `users`
  SET `password_reset_token` = NULL, `password_reset_expires_at` = NULL
  WHERE `password_reset_token` IS NOT NULL;
