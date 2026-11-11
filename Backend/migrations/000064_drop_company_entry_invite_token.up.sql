-- ゲスト投稿の招待トークンは pending_registrations で検証する。
-- company_entry_submissions.invite_token は読む箇所が無く、平文のまま残っていた（#1990）。
-- company_users の 000020 と同じく、値は戻せないので列ごと落とす。

UPDATE `company_entry_submissions` SET `invite_token` = NULL
  WHERE `invite_token` IS NOT NULL;

ALTER TABLE `company_entry_submissions` DROP COLUMN `invite_token`;
