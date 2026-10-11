-- 検査由来のスコア行を先に消してから source 列を落とす。
-- 逆にすると由来が分からなくなり、検査の行がチャットの行として残る（#1678）。
DELETE FROM `user_weight_scores` WHERE `source` = 'assessment';
ALTER TABLE `user_weight_scores` DROP COLUMN `source`;

DROP TABLE IF EXISTS `assessment_results`;
DROP TABLE IF EXISTS `assessment_answers`;
DROP TABLE IF EXISTS `assessment_sessions`;
DROP TABLE IF EXISTS `assessment_questions`;
DROP TABLE IF EXISTS `assessment_templates`;
