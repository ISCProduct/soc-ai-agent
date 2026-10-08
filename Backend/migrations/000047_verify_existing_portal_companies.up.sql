-- 企業ポータルの学生検索・スカウトは companies.is_verified が true になるまで拒否する。
-- このフラグを立てる管理操作が無かったため、既存のポータル利用者は全員止まる。
-- すでに担当者アカウントがある企業は、この変更より前から学生検索を使えていたものとして審査済みにする。
-- これ以降の自己登録は is_verified=false のままなので、管理画面の審査完了が必要。

UPDATE `companies` AS c
INNER JOIN (
  SELECT DISTINCT `company_id` FROM `company_users`
) AS portal ON portal.company_id = c.id
SET c.is_verified = 1
WHERE c.is_verified = 0;
