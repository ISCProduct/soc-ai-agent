-- 学生のパスワード再設定トークンを、平文ではなく SHA-256 の hex で持つ（SOCAIAGENT-693）。
-- 企業ユーザー側（000020 の password_reset_token_hash）に揃える。
--
-- 段階1: 新しい列を足し、コードは新しい列だけを読み書きする。
-- 旧列 password_reset_token は、デプロイ中に旧タスクが読むので、この段階では消さない。
-- 段階2（後続の PR）で旧列を削除する。
ALTER TABLE `users`
  ADD COLUMN `password_reset_token_hash` varchar(64) DEFAULT NULL,
  ALGORITHM=INSTANT;

ALTER TABLE `users`
  ADD UNIQUE KEY `uk_users_password_reset_token_hash` (`password_reset_token_hash`),
  ALGORITHM=INPLACE, LOCK=NONE;
