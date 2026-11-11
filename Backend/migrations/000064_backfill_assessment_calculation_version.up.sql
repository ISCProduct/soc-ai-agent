-- 適性検査のスコアはチャットから再構築しないので、最初から現行の計算版として扱う（#1770）。
UPDATE `user_weight_scores`
SET `calculation_version` = 'v1'
WHERE `source` = 'assessment';
