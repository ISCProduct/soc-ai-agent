-- シード(seed_company_relations.go)にハードコードされていた法人番号のうち 8 件が
-- 登記に存在しない番号だった。国税庁「法人番号システムWeb-API」で実在確認した
-- 正しい番号へ前方修正する。
--
-- これらの行は SourceType='public_registry' / IsProvisional=0 で作られており、
-- 登記由来の確定情報として扱われていた。親会社側の法人番号が不正なため
-- gBizINFO 同期も 404 で失敗し続けていた。
--
-- 法人番号に UNIQUE 制約は無い（idx_companies_corporate_number は非ユニーク）ため
-- UPDATE で衝突しない。

UPDATE `companies` SET `corporate_number` = '1180301018771' WHERE `corporate_number` = '7010401026738'; -- トヨタ自動車
UPDATE `companies` SET `corporate_number` = '3180301014273' WHERE `corporate_number` = '4180301012460'; -- 豊田自動織機
UPDATE `companies` SET `corporate_number` = '7010401022916' WHERE `corporate_number` = '4010401019905'; -- 日本電気
UPDATE `companies` SET `corporate_number` = '4010001073486' WHERE `corporate_number` = '8010105001109'; -- 三菱UFJフィナンシャル・グループ
UPDATE `companies` SET `corporate_number` = '7010001034964' WHERE `corporate_number` = '9010401013693'; -- ヤマトホールディングス
UPDATE `companies` SET `corporate_number` = '1010001034730' WHERE `corporate_number` = '4010401010393'; -- 内田洋行
UPDATE `companies` SET `corporate_number` = '8011001058176' WHERE `corporate_number` = '3010401028474'; -- パーソルホールディングス
UPDATE `companies` SET `corporate_number` = '8010001034740' WHERE `corporate_number` = '9060001000184'; -- 味の素

-- 登記上の商号に揃える（NEC はブランド名で、登記は日本電気株式会社）。
UPDATE `companies` SET `name` = '日本電気株式会社'
WHERE `corporate_number` = '7010401022916' AND `name` = 'NEC株式会社';

-- 商号変更の反映（登記は 2026-04-01 付で株式会社オープンアップソリューションズ）。
UPDATE `companies` SET `name` = '株式会社オープンアップソリューションズ'
WHERE `corporate_number` = '5180301037157' AND `name` = '株式会社ビーネックスソリューションズ';

-- 誤った法人番号で作られた「出所: 公開情報」の資本関係を削除する。
-- seedCompanyRelations は該当行が 0 件のときだけ再投入するため、
-- 次回起動時に修正後の正しい関係が入り直す。
DELETE FROM `company_relations` WHERE `description` LIKE '%（出所: 公開情報）';

-- 失敗したまま残っている gBizINFO の同期結果をクリアする。
-- 404 応答の HTML 本文がそのまま格納されている行がある。
UPDATE `companies`
SET `g_biz_sync_status` = '', `g_biz_sync_message` = '', `g_biz_last_synced_at` = NULL
WHERE `g_biz_sync_status` = 'failed';
