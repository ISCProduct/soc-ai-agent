-- 国税庁の法人番号公表サイトで確認した、法人種別と登記の閉鎖の状態。
-- 学生向け企業相関図から、国の機関・地方公共団体と、閉鎖された企業（よく知る企業を除く）を除くために使う。
--   corporate_kind     : 法人種別コード（101=国の機関, 201=地方公共団体, 301=株式会社 など）。空は未確認
--   registry_closed_on : 登記記録の閉鎖等年月日。NULL は、閉鎖されていない、または未確認
--   registry_checked_at: 国税庁で確認した日時。NULL は未確認（「閉鎖されていない」と「未確認」を区別する）
ALTER TABLE `companies`
  ADD COLUMN `corporate_kind` VARCHAR(3) NOT NULL DEFAULT '' AFTER `corporate_number`,
  ADD COLUMN `registry_closed_on` DATE NULL AFTER `corporate_kind`,
  ADD COLUMN `registry_checked_at` DATETIME NULL AFTER `registry_closed_on`;
