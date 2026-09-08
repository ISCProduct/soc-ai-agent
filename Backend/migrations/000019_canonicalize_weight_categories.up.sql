-- カテゴリ名の表記揺れを正典へ寄せる（#929 / SOCAIAGENT-205）
--
-- 正典は domain/valueobject/match.go の10種類。
-- マッチングは scoreMap を正典キーで引くため、揺れた名前で保存された行は
-- 一度も引かれず、代わりに中立50が使われる(matching_service.go の scoredMatch)。
-- エラーもログも出ないままユーザーの実スコアが捨てられていた。
--
-- 注: develop の最新は 000018。番号が飛ぶと golang-migrate が後から埋めた分を
-- 二度と実行しないため、必ず連番にすること（migrations_test.go が検査する）。

-- user_weight_scores: マッチングの入力そのもの。実害が出ていた。
-- 「志向」サフィックスの欠落のみが確認されている。
UPDATE `user_weight_scores` SET `weight_category` = 'チームワーク志向'   WHERE `weight_category` = 'チームワーク';
UPDATE `user_weight_scores` SET `weight_category` = 'リーダーシップ志向' WHERE `weight_category` = 'リーダーシップ';
UPDATE `user_weight_scores` SET `weight_category` = '創造性志向'         WHERE `weight_category` = '創造性';
UPDATE `user_weight_scores` SET `weight_category` = 'コミュニケーション力' WHERE `weight_category` IN ('コミュニケーション', 'コミュニケーション能力');
-- 別名表(valueobject.weightCategoryAliases)と同じ範囲を必ずカバーする。
-- 取りこぼした行は、AddScore が別名を正典へ寄せるようになった今、
-- 二度と更新されない孤児行として残る。
UPDATE `user_weight_scores` SET `weight_category` = '技術志向'       WHERE `weight_category` IN ('問題解決力', '分析思考', '技術');
UPDATE `user_weight_scores` SET `weight_category` = '細部志向'       WHERE `weight_category` IN ('計画性・実行力', '細部');
UPDATE `user_weight_scores` SET `weight_category` = '成長志向'       WHERE `weight_category` IN ('学習意欲・成長志向', 'ビジネス思考・目標志向', '成長');
UPDATE `user_weight_scores` SET `weight_category` = 'チャレンジ志向' WHERE `weight_category` IN ('ストレス耐性・粘り強さ', 'チャレンジ');
UPDATE `user_weight_scores` SET `weight_category` = '創造性志向'     WHERE `weight_category` = '創造性・発想力';
UPDATE `user_weight_scores` SET `weight_category` = '安定志向'       WHERE `weight_category` = '安定';

-- 旧名と新名の両方を持っていたユーザーは、ここまでの UPDATE で
-- 同一キーの行が2本並ぶ。この表に一意制約は無く
-- (000001 の idx_user_category は非ユニーク)、読み出し側の
-- scoreMap は後勝ち、FindByUserSessionAndCategory の First は不定になる。
-- 高い方を残して重複を畳む。
DELETE t FROM `user_weight_scores` t
JOIN (
  SELECT `user_id`, `session_id`, `weight_category`, MAX(`score`) AS max_score, MIN(`id`) AS keep_id
  FROM `user_weight_scores`
  GROUP BY `user_id`, `session_id`, `weight_category`
  HAVING COUNT(*) > 1
) d
  ON t.`user_id` = d.`user_id`
 AND t.`session_id` = d.`session_id`
 AND t.`weight_category` = d.`weight_category`
WHERE t.`id` <> d.keep_id;

UPDATE `user_weight_scores` t
JOIN (
  SELECT `user_id`, `session_id`, `weight_category`, MAX(`score`) AS max_score
  FROM `user_weight_scores`
  GROUP BY `user_id`, `session_id`, `weight_category`
) d
  ON t.`user_id` = d.`user_id`
 AND t.`session_id` = d.`session_id`
 AND t.`weight_category` = d.`weight_category`
SET t.`score` = d.max_score;

-- question_weights: シード由来。質問体系の別分類がそのまま入っている。
UPDATE `question_weights` SET `weight_category` = 'コミュニケーション力' WHERE `weight_category` IN ('コミュニケーション', 'コミュニケーション能力');
UPDATE `question_weights` SET `weight_category` = 'リーダーシップ志向'   WHERE `weight_category` = 'リーダーシップ';
UPDATE `question_weights` SET `weight_category` = 'チームワーク志向'     WHERE `weight_category` = 'チームワーク';
UPDATE `question_weights` SET `weight_category` = '創造性志向'           WHERE `weight_category` IN ('創造性', '創造性・発想力');
UPDATE `question_weights` SET `weight_category` = '技術志向'             WHERE `weight_category` IN ('問題解決力', '分析思考');
UPDATE `question_weights` SET `weight_category` = '細部志向'             WHERE `weight_category` = '計画性・実行力';
UPDATE `question_weights` SET `weight_category` = '成長志向'             WHERE `weight_category` IN ('学習意欲・成長志向', 'ビジネス思考・目標志向');
UPDATE `question_weights` SET `weight_category` = 'チャレンジ志向'       WHERE `weight_category` = 'ストレス耐性・粘り強さ';

-- ai_question_templates: 選択肢回答のスコア反映に使われる（TemplateID が設定された時点で有効化される）。
UPDATE `ai_question_templates` SET `category` = 'コミュニケーション力' WHERE `category` IN ('コミュニケーション', 'コミュニケーション能力');
UPDATE `ai_question_templates` SET `category` = 'リーダーシップ志向'   WHERE `category` = 'リーダーシップ';
UPDATE `ai_question_templates` SET `category` = 'チームワーク志向'     WHERE `category` = 'チームワーク';
UPDATE `ai_question_templates` SET `category` = '創造性志向'           WHERE `category` IN ('創造性', '創造性・発想力');
UPDATE `ai_question_templates` SET `category` = '技術志向'             WHERE `category` IN ('問題解決力', '分析思考');
UPDATE `ai_question_templates` SET `category` = '細部志向'             WHERE `category` = '計画性・実行力';
UPDATE `ai_question_templates` SET `category` = '成長志向'             WHERE `category` IN ('学習意欲・成長志向', 'ビジネス思考・目標志向');
UPDATE `ai_question_templates` SET `category` = 'チャレンジ志向'       WHERE `category` = 'ストレス耐性・粘り強さ';
