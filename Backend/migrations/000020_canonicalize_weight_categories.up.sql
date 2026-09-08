-- カテゴリ名の表記揺れを正典へ寄せる（#929 / SOCAIAGENT-205）
--
-- 正典は domain/valueobject/match.go の10種類。
-- マッチングは scoreMap を正典キーで引くため、揺れた名前で保存された行は
-- 一度も引かれず、代わりに中立50が使われる(matching_service.go の scoredMatch)。
-- エラーもログも出ないままユーザーの実スコアが捨てられていた。
--
-- 注: 000019 は #1196(企業ユーザーの復旧・剥奪)が使用しているため 000020 とする。

-- user_weight_scores: マッチングの入力そのもの。実害が出ていた。
-- 「志向」サフィックスの欠落のみが確認されている。
UPDATE `user_weight_scores` SET `weight_category` = 'チームワーク志向'   WHERE `weight_category` = 'チームワーク';
UPDATE `user_weight_scores` SET `weight_category` = 'リーダーシップ志向' WHERE `weight_category` = 'リーダーシップ';
UPDATE `user_weight_scores` SET `weight_category` = '創造性志向'         WHERE `weight_category` = '創造性';
UPDATE `user_weight_scores` SET `weight_category` = 'コミュニケーション力' WHERE `weight_category` IN ('コミュニケーション', 'コミュニケーション能力');

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
