-- 能力検査の機能ごと撤去する。テーブルを落とすので、残した行は参照先を失う（孤児になる）。
-- 外部キーの向きがある順に落とす。
DROP TABLE IF EXISTS `ability_results`;
DROP TABLE IF EXISTS `ability_answers`;
DROP TABLE IF EXISTS `ability_sessions`;
DROP TABLE IF EXISTS `ability_questions`;
DROP TABLE IF EXISTS `ability_templates`;
