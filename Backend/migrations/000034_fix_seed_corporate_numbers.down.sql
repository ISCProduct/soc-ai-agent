-- 誤った法人番号へ戻す。登記に存在しない番号のため、通常は戻す理由が無い。
UPDATE `companies` SET `corporate_number` = '7010401026738' WHERE `corporate_number` = '1180301018771';
UPDATE `companies` SET `corporate_number` = '4180301012460' WHERE `corporate_number` = '3180301014273';
UPDATE `companies` SET `corporate_number` = '4010401019905' WHERE `corporate_number` = '7010401022916';
UPDATE `companies` SET `corporate_number` = '8010105001109' WHERE `corporate_number` = '4010001073486';
UPDATE `companies` SET `corporate_number` = '9010401013693' WHERE `corporate_number` = '7010001034964';
UPDATE `companies` SET `corporate_number` = '4010401010393' WHERE `corporate_number` = '1010001034730';
UPDATE `companies` SET `corporate_number` = '3010401028474' WHERE `corporate_number` = '8011001058176';
UPDATE `companies` SET `corporate_number` = '9060001000184' WHERE `corporate_number` = '8010001034740';

UPDATE `companies` SET `name` = 'NEC株式会社'
WHERE `corporate_number` = '7010401022916' AND `name` = '日本電気株式会社';

UPDATE `companies` SET `name` = '株式会社ビーネックスソリューションズ'
WHERE `corporate_number` = '5180301037157' AND `name` = '株式会社オープンアップソリューションズ';

-- 削除した資本関係はシードが再投入するため、ここでは復元しない。
-- 同期結果のクリアも復元しない（次回同期で埋まる）。
