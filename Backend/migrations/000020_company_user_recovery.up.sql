-- 企業ユーザーの復旧手段とアクセス剥奪（#1196）
--
-- パスワードを忘れた企業ユーザーは、リセット導線が無く admin の再招待も
-- ErrEmailExists で弾かれるため復旧不能だった。あわせて退職者のアクセスを
-- 止める手段が行削除しか無く、削除するとその担当者が付けた自社タグが
-- ON DELETE CASCADE で消えるデータ損失があった。

-- パスワードリセット。トークンは平文で持たずSHA-256のhexを保存する。
ALTER TABLE `company_users`
  ADD COLUMN `password_reset_token_hash` varchar(64) DEFAULT NULL,
  ADD COLUMN `password_reset_expires_at` datetime(3) DEFAULT NULL,
  -- 無効化。行を消さずにアクセスを剥奪するための列。
  ADD COLUMN `disabled_at` datetime(3) DEFAULT NULL,
  -- 招待トークンもハッシュ化する。リフレッシュトークンが SHA-256 で
  -- 保存されているのに招待だけ平文なのは非対称だった。
  ADD COLUMN `invite_token_hash` varchar(64) DEFAULT NULL;

ALTER TABLE `company_users`
  ADD UNIQUE KEY `uk_company_users_password_reset_hash` (`password_reset_token_hash`),
  ADD UNIQUE KEY `uk_company_users_invite_token_hash` (`invite_token_hash`);

-- 既存の平文招待トークンは移行できない（ハッシュから平文は復元できず、
-- 平文をハッシュ化してもメール中のリンクとは対応が取れるが、平文が
-- DBに残る期間を無くすことを優先する）。未受諾の招待は失効させ、
-- 再招待で発行し直す運用とする。再招待は本PRで可能になる。
UPDATE `company_users` SET `invite_token` = NULL, `invite_expires_at` = NULL
  WHERE `invite_token` IS NOT NULL;

ALTER TABLE `company_users` DROP COLUMN `invite_token`;

-- 企業タグは企業に属するデータであって作成者個人のものではない。
-- 担当者の削除でタグが消えないよう RESTRICT にする。
-- アクセス剥奪は disabled_at で行うため、削除を止めても運用は回る。
ALTER TABLE `company_student_tags`
  DROP FOREIGN KEY `fk_company_student_tags_creator`;
ALTER TABLE `company_student_tags`
  ADD CONSTRAINT `fk_company_student_tags_creator`
  FOREIGN KEY (`created_by`) REFERENCES `company_users` (`id`) ON DELETE RESTRICT;
