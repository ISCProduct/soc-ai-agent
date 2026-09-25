-- ゲスト投稿由来かどうかを企業行そのものに持たせる(#1409)。
--
-- guestEntryVisibilityGuard は company_entry_submissions に行があることを
-- 「ゲスト投稿である」唯一の識別子にしていた。この行が消えると
-- 「ゲスト投稿ではない」と判定され、審査前の企業が未認証の公開APIへ出る
-- (fail-open)。誰でも無認証で投稿できるため、任意の内容を学生へ露出させられる。
--
-- 外部キーでは塞げない。companies -> company_entry_submissions の FK は
-- 「企業を消せない」制約であって、投稿行そのものの削除は防げない。
-- 表示可否の判断材料を監査用の別テーブルに置いていたことが原因なので、
-- 企業行へ移す。
ALTER TABLE `companies`
  ADD COLUMN `is_guest_entry` BOOLEAN NOT NULL DEFAULT 0 AFTER `is_provisional`;

-- 既存のゲスト投稿を埋める。ここで取りこぼすと、その企業だけガードが外れる。
UPDATE `companies` c
  JOIN `company_entry_submissions` s ON s.company_id = c.id
SET c.is_guest_entry = 1;

-- 学生向けの一覧はこの列で絞るため、単体で引けるようにする。
CREATE INDEX `idx_companies_guest_entry` ON `companies` (`is_guest_entry`);
